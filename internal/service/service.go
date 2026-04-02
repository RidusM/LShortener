package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode"

	"lshortener/internal/entity"
	"lshortener/pkg/keygen"

	"github.com/google/uuid"
	pgxdriver "github.com/wb-go/wbf/dbpg/pgx-driver"
	"github.com/wb-go/wbf/dbpg/pgx-driver/transaction"
	"github.com/wb-go/wbf/logger"
)

const (
	_defaultShortCodeLength = 6
	_defaultMaxRetries      = 10
	_slowOperationThreshold = 200 * time.Millisecond
	_defaultAnalyticsLimit  = 50

	_contextTimeout = 5 * time.Second
)

var (
	ErrURLNotFound        = errors.New("url not found")
	ErrURLExpired         = errors.New("url expired")
	ErrURLInactive        = errors.New("url inactive")
	ErrAliasAlreadyExists = errors.New("alias already exists")
	ErrInvalidURL         = errors.New("invalid url format")
)

type (
	URLRepository interface {
		Create(ctx context.Context, qe pgxdriver.QueryExecuter, url entity.URL) (*entity.URL, error)
		GetByShortCode(ctx context.Context, qe pgxdriver.QueryExecuter, shortCode string) (*entity.URL, error)
		GetByCustomAlias(ctx context.Context, qe pgxdriver.QueryExecuter, alias string) (*entity.URL, error)
		IncrementClickCount(ctx context.Context, qe pgxdriver.QueryExecuter, urlID uuid.UUID) error
		ShortCodeExists(ctx context.Context, qe pgxdriver.QueryExecuter, shortCode string) (bool, error)
		CustomAliasExists(ctx context.Context, qe pgxdriver.QueryExecuter, alias string) (bool, error)
	}

	AnalyticsRepository interface {
		RecordClick(ctx context.Context, qe pgxdriver.QueryExecuter, analytics entity.Analytics) error
		GetStatsByShortCode(
			ctx context.Context,
			qe pgxdriver.QueryExecuter,
			shortCode string,
			limit uint64,
		) (*entity.AnalyticsStats, error)
	}

	CacheRepository interface {
		Get(ctx context.Context, shortCode string) (*entity.URL, error)
		Save(ctx context.Context, url *entity.URL) error
		Invalidate(ctx context.Context, shortCode string) error
		IncrementClickCount(ctx context.Context, shortCode string) error
	}

	ShortenerService struct {
		urlRepo       URLRepository
		analyticsRepo AnalyticsRepository
		cache         CacheRepository
		tm            transaction.Manager
		log           logger.Logger
		keygen        keygen.Generator

		codeLen    int
		defaultTTL time.Duration
		baseURL    string
		maxRetries int
	}

	CreateURLRequest struct {
		OriginalURL string
		CustomAlias *string
		ExpiresAt   *time.Time
	}

	CreateURLResponse struct {
		ID          uuid.UUID  `json:"id"`
		ShortCode   string     `json:"short_code"`
		ShortURL    string     `json:"short_url"`
		OriginalURL string     `json:"original_url"`
		CustomAlias *string    `json:"custom_alias,omitempty"`
		ExpiresAt   *time.Time `json:"expires_at,omitempty"`
		CreatedAt   time.Time  `json:"created_at"`
	}

	ClickInfo struct {
		UserAgent string
		IPAddress string
		Referer   string
	}
)

func NewShortenerService(
	urlRepo URLRepository,
	analyticsRepo AnalyticsRepository,
	cache CacheRepository,
	tm transaction.Manager,
	log logger.Logger,
	kg keygen.Generator,
	opts ...Option,
) (*ShortenerService, error) {
	s := &ShortenerService{
		urlRepo:       urlRepo,
		analyticsRepo: analyticsRepo,
		cache:         cache,
		tm:            tm,
		log:           log,
		keygen:        kg,
		codeLen:       _defaultShortCodeLength,
		maxRetries:    _defaultMaxRetries,
	}

	for _, opt := range opts {
		opt(s)
	}

	if err := s.validate(); err != nil {
		return nil, fmt.Errorf("service.NewShotrtenerService: %w", err)
	}

	return s, nil
}

func (s *ShortenerService) CreateShortURL(ctx context.Context, req CreateURLRequest) (*CreateURLResponse, error) {
	const op = "service.CreateShortURL"

	log := s.log.Ctx(ctx).With("op", op)
	startTime := time.Now()

	defer s.logSlowOperation(
		ctx,
		op,
		startTime,
		logger.String("original_url", req.OriginalURL),
		logger.Bool("has_alias", req.CustomAlias != nil),
	)

	log.LogAttrs(ctx, logger.InfoLevel, "create short url started",
		logger.String("original_url", req.OriginalURL),
	)

	if err := s.validateURL(req.OriginalURL); err != nil {
		log.LogAttrs(ctx, logger.ErrorLevel, "invalid url",
			logger.Any("error", err),
		)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	if req.CustomAlias != nil {
		if err := s.validateCustomAlias(*req.CustomAlias); err != nil {
			log.LogAttrs(ctx, logger.ErrorLevel, "invalid custom alias",
				logger.Any("error", err),
			)
			return nil, fmt.Errorf("%s: %w", op, err)
		}
	}

	var result *entity.URL
	err := s.tm.ExecuteInTransaction(ctx, "create_short_url", func(tx pgxdriver.QueryExecuter) error {
		if req.CustomAlias != nil {
			exists, err := s.urlRepo.CustomAliasExists(ctx, tx, *req.CustomAlias)
			if err != nil {
				return fmt.Errorf("%s: alias exists: %w", op, err)
			}
			if exists {
				return ErrAliasAlreadyExists
			}
		}

		shortCode, err := s.generateUniqueShortCode(ctx, tx)
		if err != nil {
			return fmt.Errorf("%s: short code: %w", op, err)
		}

		var expiresAt *time.Time
		if req.ExpiresAt != nil {
			expiresAt = req.ExpiresAt
		} else if s.defaultTTL > 0 {
			exp := time.Now().UTC().Add(s.defaultTTL)
			expiresAt = &exp
		}

		urlEntity := entity.URL{
			ShortCode:   shortCode,
			OriginalURL: req.OriginalURL,
			CustomAlias: req.CustomAlias,
			ExpiresAt:   expiresAt,
		}

		created, err := s.urlRepo.Create(ctx, tx, urlEntity)
		if err != nil {
			return transaction.HandleError("create_short_url", "create", err)
		}

		result = created
		return nil
	})
	if err != nil {
		log.LogAttrs(ctx, logger.ErrorLevel, "creation failed",
			logger.Any("error", err),
		)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	_ = s.cache.Save(ctx, result)

	response := &CreateURLResponse{
		ID:          result.ID,
		ShortCode:   result.ShortCode,
		ShortURL:    s.buildShortURL(result.ShortCode),
		OriginalURL: result.OriginalURL,
		CustomAlias: result.CustomAlias,
		ExpiresAt:   result.ExpiresAt,
		CreatedAt:   entity.ExtractTimestampFromUUIDv7(result.ID),
	}

	log.LogAttrs(ctx, logger.InfoLevel, "short url created",
		logger.String("short_code", result.ShortCode),
		logger.Duration("duration", time.Since(startTime)),
	)

	return response, nil
}

func (s *ShortenerService) ResolveShortURL(ctx context.Context, shortCode string, clickInfo ClickInfo) (string, error) {
	const op = "service.ResolveShortURL"

	log := s.log.Ctx(ctx).With("op", op)
	startTime := time.Now()

	defer s.logSlowOperation(ctx, op, startTime, logger.String("short_code", shortCode))

	log.LogAttrs(ctx, logger.InfoLevel, "resolve short url started",
		logger.String("short_code", shortCode),
	)

	cachedURL, err := s.cache.Get(ctx, shortCode)
	if err == nil && cachedURL != nil {
		if err = s.validateURLAccess(cachedURL); err != nil {
			log.LogAttrs(ctx, logger.WarnLevel, "url access denied from cache",
				logger.Any("error", err),
			)
			return "", fmt.Errorf("%s: %w", op, err)
		}

		go func() {
			bgCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), _contextTimeout)
			defer cancel()

			if err = s.recordClickAndIncrement(bgCtx, cachedURL.ID, shortCode, clickInfo); err != nil {
				s.log.Ctx(bgCtx).LogAttrs(bgCtx, logger.ErrorLevel, "failed to record click async",
					logger.String("short_code", shortCode),
					logger.Any("error", err),
				)
			}
		}()
		log.LogAttrs(ctx, logger.InfoLevel, "url resolved from cache",
			logger.String("short_code", shortCode),
			logger.Duration("duration", time.Since(startTime)),
		)

		return cachedURL.OriginalURL, nil
	}

	var originalURL string
	err = s.tm.ExecuteInTransaction(ctx, "resolve_short_url", func(tx pgxdriver.QueryExecuter) error {
		urlEntity, getErr := s.urlRepo.GetByShortCode(ctx, tx, shortCode)
		if getErr != nil {
			if errors.Is(getErr, entity.ErrDataNotFound) {
				return ErrURLNotFound
			}
			return fmt.Errorf("%s: short code: %w", op, getErr)
		}

		if err = s.validateURLAccess(urlEntity); err != nil {
			return err
		}

		_ = s.cache.Save(ctx, urlEntity)

		analytics := entity.Analytics{
			URLId:     urlEntity.ID,
			UserAgent: clickInfo.UserAgent,
			IPAddress: clickInfo.IPAddress,
			Referer:   clickInfo.Referer,
		}

		if err = s.analyticsRepo.RecordClick(ctx, tx, analytics); err != nil {
			log.LogAttrs(ctx, logger.ErrorLevel, "failed to record click",
				logger.Any("error", err),
			)
		}

		if err = s.urlRepo.IncrementClickCount(ctx, tx, urlEntity.ID); err != nil {
			log.LogAttrs(ctx, logger.ErrorLevel, "failed to increment click count",
				logger.Any("error", err),
			)
		}

		originalURL = urlEntity.OriginalURL
		return nil
	})
	if err != nil {
		log.LogAttrs(ctx, logger.ErrorLevel, "resolve failed",
			logger.Any("error", err),
		)
		return "", fmt.Errorf("%s: %w", op, err)
	}

	log.LogAttrs(ctx, logger.InfoLevel, "url resolved from database",
		logger.String("short_code", shortCode),
		logger.Duration("duration", time.Since(startTime)),
	)

	return originalURL, nil
}

func (s *ShortenerService) GetAnalytics(ctx context.Context, shortCode string) (*entity.AnalyticsStats, error) {
	const op = "service.GetAnalytics"

	log := s.log.Ctx(ctx).With("op", op)
	startTime := time.Now()

	defer s.logSlowOperation(ctx, op, startTime, logger.String("short_code", shortCode))

	log.LogAttrs(ctx, logger.InfoLevel, "get analytics started",
		logger.String("short_code", shortCode),
	)

	var stats *entity.AnalyticsStats
	err := s.tm.ExecuteInTransaction(ctx, "get_analytics", func(tx pgxdriver.QueryExecuter) error {
		var err error
		stats, err = s.analyticsRepo.GetStatsByShortCode(ctx, tx, shortCode, _defaultAnalyticsLimit)
		if err != nil {
			if errors.Is(err, entity.ErrDataNotFound) {
				return ErrURLNotFound
			}
			return fmt.Errorf("%s: stats: %w", op, err)
		}
		return nil
	})
	if err != nil {
		log.LogAttrs(ctx, logger.ErrorLevel, "get analytics failed",
			logger.Any("error", err),
		)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	log.LogAttrs(ctx, logger.InfoLevel, "analytics retrieved",
		logger.String("short_code", shortCode),
		logger.Int64("total_clicks", stats.TotalClicks),
		logger.Duration("duration", time.Since(startTime)),
	)

	return stats, nil
}

func (s *ShortenerService) generateUniqueShortCode(ctx context.Context, tx pgxdriver.QueryExecuter) (string, error) {
	for range s.maxRetries {
		shortCode, err := s.keygen.Generate(s.codeLen)
		if err != nil {
			return "", fmt.Errorf("keygen failed: %w", err)
		}

		exists, err := s.urlRepo.ShortCodeExists(ctx, tx, shortCode)
		if err != nil {
			return "", fmt.Errorf("check exists: %w", err)
		}

		if !exists {
			return shortCode, nil
		}
	}

	return "", errors.New("failed to generate unique short code after max retries")
}

func (s *ShortenerService) validateURL(urlStr string) error {
	if urlStr == "" {
		return fmt.Errorf("url is required: %w", ErrInvalidURL)
	}

	parsedURL, err := url.ParseRequestURI(urlStr)
	if err != nil {
		return fmt.Errorf("invalid url format: %w", ErrInvalidURL)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("url must use http or https scheme: %w", ErrInvalidURL)
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("url must have a host: %w", ErrInvalidURL)
	}

	return nil
}

func (s *ShortenerService) validateCustomAlias(alias string) error {
	if alias == "" {
		return fmt.Errorf("alias cannot be empty: %w", entity.ErrInvalidData)
	}

	if len(alias) < 3 || len(alias) > 50 {
		return fmt.Errorf("alias must be between 3 and 50 characters: %w", entity.ErrInvalidData)
	}

	for _, char := range alias {
		if !unicode.IsLetter(char) &&
			!unicode.IsDigit(char) &&
			char != '-' &&
			char != '_' {
			return fmt.Errorf("alias contains invalid characters: %w", entity.ErrInvalidData)
		}
	}

	reserved := []string{"api", "admin", "analytics", "health", "shorten", "s"}
	aliasLower := strings.ToLower(alias)
	if slices.Contains(reserved, aliasLower) {
		return fmt.Errorf("alias is reserved: %w", entity.ErrInvalidData)
	}

	return nil
}

func (s *ShortenerService) validateURLAccess(url *entity.URL) error {
	if !url.IsActive {
		return ErrURLInactive
	}

	if url.ExpiresAt != nil && url.ExpiresAt.Before(time.Now().UTC()) {
		return ErrURLExpired
	}

	return nil
}

func (s *ShortenerService) buildShortURL(shortCode string) string {
	if s.baseURL == "" {
		return shortCode
	}
	return fmt.Sprintf("%s/s/%s", strings.TrimSuffix(s.baseURL, "/"), shortCode)
}

func (s *ShortenerService) recordClickAndIncrement(
	ctx context.Context,
	urlID uuid.UUID,
	shortCode string,
	clickInfo ClickInfo,
) error {
	if err := s.tm.ExecuteInTransaction(ctx, "record_click", func(tx pgxdriver.QueryExecuter) error {
		analytics := entity.Analytics{
			URLId:     urlID,
			UserAgent: clickInfo.UserAgent,
			IPAddress: clickInfo.IPAddress,
			Referer:   clickInfo.Referer,
		}

		if err := s.analyticsRepo.RecordClick(ctx, tx, analytics); err != nil {
			return fmt.Errorf("record click: %w", err)
		}

		if err := s.urlRepo.IncrementClickCount(ctx, tx, urlID); err != nil {
			return fmt.Errorf("increment click count: %w", err)
		}

		_ = s.cache.IncrementClickCount(ctx, shortCode)

		return nil
	}); err != nil {
		return fmt.Errorf("service.recordClickAndIncrement: %w", err)
	}
	return nil
}

func (s *ShortenerService) logSlowOperation(
	ctx context.Context,
	op string,
	startTime time.Time,
	attrs ...logger.Attr,
) {
	duration := time.Since(startTime)
	if duration > _slowOperationThreshold {
		allAttrs := append([]logger.Attr{
			logger.String("op", op),
			logger.Duration("duration", duration),
		}, attrs...)
		s.log.Ctx(ctx).LogAttrs(ctx, logger.WarnLevel, "slow operation detected", allAttrs...)
	}
}
