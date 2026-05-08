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
	_defaultRetryDelay      = 5 * time.Millisecond
	_defaultQueryLimit      = 50

	_slowOperationThreshold = 200 * time.Millisecond
	_asyncClickTimeout      = 5 * time.Second
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
		GetURLInfoByShortCode(ctx context.Context, qe pgxdriver.QueryExecuter, shortCode string) (*entity.URL, error)
		GetClicksByDay(
			ctx context.Context,
			qe pgxdriver.QueryExecuter,
			urlID uuid.UUID,
			limit uint64,
		) (map[string]int64, error)
		GetClicksByUA(
			ctx context.Context,
			qe pgxdriver.QueryExecuter,
			urlID uuid.UUID,
			limit uint64,
		) (map[string]int64, error)
		GetRecentClicks(
			ctx context.Context,
			qe pgxdriver.QueryExecuter,
			urlID uuid.UUID,
			limit uint64,
		) ([]entity.Analytics, error)
	}

	CacheRepository interface {
		Get(ctx context.Context, shortCode string) (*entity.URL, error)
		Save(ctx context.Context, url *entity.URL) error
		Invalidate(ctx context.Context, shortCode string) error
		IncrementClickCount(ctx context.Context, shortCode string) error
	}

	CreateURLRequest struct {
		OriginalURL string
		CustomAlias *string
		ExpiresAt   *time.Time
	}

	CreateURLResponse struct {
		ID          uuid.UUID
		ShortCode   string
		ShortURL    string
		OriginalURL string
		CustomAlias *string
		ExpiresAt   *time.Time
		CreatedAt   time.Time
	}

	ClickInfo struct {
		UserAgent string
		IPAddress string
		Referer   string
		ClickedAt time.Time
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
		retryDelay time.Duration
		queryLimit uint64
	}
)

func NewShortenerService(
	urlRepo URLRepository,
	analyticsRepo AnalyticsRepository,
	cache CacheRepository,
	tm transaction.Manager,
	kg keygen.Generator,
	log logger.Logger,
	opts ...Option,
) *ShortenerService {
	s := &ShortenerService{
		urlRepo:       urlRepo,
		analyticsRepo: analyticsRepo,
		cache:         cache,
		tm:            tm,
		keygen:        kg,
		log:           log,
		codeLen:       _defaultShortCodeLength,
		maxRetries:    _defaultMaxRetries,
		retryDelay:    _defaultRetryDelay,
		queryLimit:    _defaultQueryLimit,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *ShortenerService) CreateShortURL(ctx context.Context, req CreateURLRequest) (*CreateURLResponse, error) {
	const op = "service.CreateShortURL"

	log := s.log.With("op", op)
	startTime := time.Now()
	defer s.logSlowOperation(ctx, op, startTime,
		logger.String("original_url", req.OriginalURL),
		logger.Bool("has_alias", req.CustomAlias != nil),
	)

	log.LogAttrs(ctx, logger.InfoLevel, "create short url started",
		logger.String("original_url", req.OriginalURL),
	)

	if err := s.validateCreateRequest(req); err != nil {
		log.LogAttrs(ctx, logger.ErrorLevel, "invalid request", logger.Any("error", err))
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	var result *entity.URL
	err := s.tm.ExecuteInTransaction(ctx, "create_short_url", func(tx pgxdriver.QueryExecuter) error {
		var shortCode string
		var err error

		if req.CustomAlias != nil {
			exists, repoErr := s.urlRepo.CustomAliasExists(ctx, tx, *req.CustomAlias)
			if repoErr != nil {
				return transaction.HandleError(repoErr)
			}
			if exists {
				return entity.ErrAliasAlreadyExists
			}
			shortCode = *req.CustomAlias
		} else {
			shortCode, err = s.generateUniqueShortCode(ctx, tx)
			if err != nil {
				return fmt.Errorf("generate short code: %w", err)
			}
		}

		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("generate id: %w", err)
		}

		expiresAt := s.calculateExpiresAt(req.ExpiresAt)

		urlEntity := entity.URL{
			ID:          id,
			ShortCode:   shortCode,
			OriginalURL: req.OriginalURL,
			CustomAlias: req.CustomAlias,
			ExpiresAt:   expiresAt,
		}

		created, err := s.urlRepo.Create(ctx, tx, urlEntity)
		if err != nil {
			return transaction.HandleError(err)
		}

		result = created
		return nil
	})
	if err != nil {
		log.LogAttrs(ctx, logger.ErrorLevel, "creation failed", logger.Any("error", err))
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	if cacheErr := s.cache.Save(ctx, result); cacheErr != nil {
		log.LogAttrs(ctx, logger.WarnLevel, "cache save failed",
			logger.String("short_code", result.ShortCode),
			logger.Any("error", cacheErr),
		)
	}

	response := &CreateURLResponse{
		ID:          result.ID,
		ShortCode:   result.ShortCode,
		ShortURL:    s.buildShortURL(result.ShortCode),
		OriginalURL: result.OriginalURL,
		CustomAlias: result.CustomAlias,
		ExpiresAt:   result.ExpiresAt,
		CreatedAt:   result.CreatedAt,
	}

	log.LogAttrs(ctx, logger.InfoLevel, "short url created",
		logger.String("short_code", result.ShortCode),
		logger.Duration("duration", time.Since(startTime)),
	)
	return response, nil
}

func (s *ShortenerService) ResolveShortURL(ctx context.Context, shortCode string, clickInfo ClickInfo) (string, error) {
	const op = "service.ResolveShortURL"

	log := s.log.With("op", op)
	startTime := time.Now()
	defer s.logSlowOperation(ctx, op, startTime, logger.String("short_code", shortCode))

	log.LogAttrs(ctx, logger.InfoLevel, "resolve short url started", logger.String("short_code", shortCode))

	cachedURL, err := s.cache.Get(ctx, shortCode)
	if err == nil && cachedURL != nil {
		if err = s.validateURLAccess(cachedURL); err != nil {
			log.LogAttrs(ctx, logger.WarnLevel, "url access denied from cache", logger.Any("error", err))
			return "", fmt.Errorf("%s: %w", op, err)
		}

		go func() {
			bgCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), _asyncClickTimeout)
			defer cancel()

			s.log.Ctx(bgCtx).LogAttrs(bgCtx, logger.InfoLevel, "starting async click record",
				logger.String("short_code", shortCode),
				logger.Any("url_id", cachedURL.ID),
			)

			if err = s.recordClickAndIncrement(bgCtx, cachedURL.ID, shortCode, clickInfo); err != nil {
				s.log.Ctx(bgCtx).LogAttrs(bgCtx, logger.ErrorLevel, "failed to record click async",
					logger.String("short_code", shortCode),
					logger.Any("error", err),
				)
			} else {
				s.log.Ctx(bgCtx).LogAttrs(bgCtx, logger.InfoLevel, "async click recorded successfully",
					logger.String("short_code", shortCode),
				)
			}
		}()

		log.LogAttrs(ctx, logger.InfoLevel, "url resolved from cache",
			logger.String("short_code", shortCode),
			logger.Duration("duration", time.Since(startTime)),
		)
		return cachedURL.OriginalURL, nil
	}

	originalURL, err := s.resolveFromDB(ctx, shortCode, clickInfo, log)
	if err != nil {
		log.LogAttrs(ctx, logger.ErrorLevel, "resolve failed", logger.Any("error", err))
		return "", fmt.Errorf("%s: %w", op, err)
	}

	log.LogAttrs(ctx, logger.InfoLevel, "url resolved from database",
		logger.String("short_code", shortCode),
		logger.Duration("duration", time.Since(startTime)),
	)
	return originalURL, nil
}

func (s *ShortenerService) resolveFromDB(
	ctx context.Context,
	shortCode string,
	clickInfo ClickInfo,
	log logger.Logger,
) (string, error) {
	const op = "service.resolveFromDB"

	var originalURL string

	err := s.tm.ExecuteInTransaction(ctx, "resolve_short_url", func(tx pgxdriver.QueryExecuter) error {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("generate id: %w", err)
		}

		urlEntity, err := s.urlRepo.GetByShortCode(ctx, tx, shortCode)
		if err != nil {
			if errors.Is(err, entity.ErrDataNotFound) {
				return entity.ErrURLNotFound
			}
			return fmt.Errorf("get by short code: %w", err)
		}

		if err = s.validateURLAccess(urlEntity); err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}

		if cacheErr := s.cache.Save(ctx, urlEntity); cacheErr != nil {
			log.LogAttrs(ctx, logger.WarnLevel, "cache save failed",
				logger.String("short_code", shortCode),
				logger.Any("error", cacheErr),
			)
			return fmt.Errorf("%s: %w", op, cacheErr)
		}

		analytics := entity.Analytics{
			ID:        id,
			URLID:     urlEntity.ID,
			UserAgent: clickInfo.UserAgent,
			IPAddress: clickInfo.IPAddress,
			Referer:   clickInfo.Referer,
			ClickedAt: time.Now(),
		}

		if err = s.analyticsRepo.RecordClick(ctx, tx, analytics); err != nil {
			log.LogAttrs(ctx, logger.ErrorLevel, "failed to record click", logger.Any("error", err))
			return transaction.HandleError(err)
		}

		if err = s.urlRepo.IncrementClickCount(ctx, tx, urlEntity.ID); err != nil {
			log.LogAttrs(ctx, logger.ErrorLevel, "failed to increment click count", logger.Any("error", err))
			return transaction.HandleError(err)
		}

		originalURL = urlEntity.OriginalURL
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("%s: %w", op, err)
	}

	return originalURL, nil
}

func (s *ShortenerService) GetAnalytics(ctx context.Context, shortCode string) (*entity.AnalyticsStats, error) {
	const op = "service.GetAnalytics"

	log := s.log.With("op", op)
	startTime := time.Now()
	defer s.logSlowOperation(ctx, op, startTime, logger.String("short_code", shortCode))

	log.LogAttrs(ctx, logger.InfoLevel, "get analytics started",
		logger.String("short_code", shortCode),
	)

	var stats *entity.AnalyticsStats

	err := s.tm.ExecuteInTransaction(ctx, "get_analytics", func(tx pgxdriver.QueryExecuter) error {
		urlInfo, err := s.analyticsRepo.GetURLInfoByShortCode(ctx, tx, shortCode)
		if err != nil {
			if errors.Is(err, entity.ErrDataNotFound) {
				return entity.ErrURLNotFound
			}
			return fmt.Errorf("get url info: %w", err)
		}

		clicksByDay, err := s.analyticsRepo.GetClicksByDay(ctx, tx, urlInfo.ID, s.queryLimit)
		if err != nil {
			return fmt.Errorf("get clicks by day: %w", err)
		}

		clicksByUA, err := s.analyticsRepo.GetClicksByUA(ctx, tx, urlInfo.ID, s.queryLimit)
		if err != nil {
			return fmt.Errorf("get clicks by ua: %w", err)
		}

		recentClicks, err := s.analyticsRepo.GetRecentClicks(ctx, tx, urlInfo.ID, s.queryLimit)
		if err != nil {
			return fmt.Errorf("get recent clicks: %w", err)
		}

		stats = &entity.AnalyticsStats{
			ShortCode:    urlInfo.ShortCode,
			OriginalURL:  urlInfo.OriginalURL,
			TotalClicks:  urlInfo.ClickCount,
			ClicksByDay:  clicksByDay,
			ClicksByUA:   clicksByUA,
			RecentClicks: recentClicks,
			CreatedAt:    urlInfo.CreatedAt,
		}

		return nil
	})
	if err != nil {
		log.LogAttrs(ctx, logger.ErrorLevel, "get analytics failed", logger.Any("error", err))
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
	for attempt := range s.maxRetries {
		if attempt > 0 {
			time.Sleep(s.retryDelay)
		}

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

	return "", fmt.Errorf("failed to generate unique short code after %d attempts", s.maxRetries)
}

func (s *ShortenerService) calculateExpiresAt(reqExpiresAt *time.Time) *time.Time {
	if reqExpiresAt != nil {
		return reqExpiresAt
	}
	if s.defaultTTL > 0 {
		exp := time.Now().Add(s.defaultTTL)
		return &exp
	}
	return nil
}

func (s *ShortenerService) validateCreateRequest(req CreateURLRequest) error {
	if err := s.validateURL(req.OriginalURL); err != nil {
		return err
	}
	if req.CustomAlias != nil {
		return s.validateCustomAlias(*req.CustomAlias)
	}
	return nil
}

func (s *ShortenerService) validateURL(urlStr string) error {
	if urlStr == "" {
		return fmt.Errorf("url is required: %w", entity.ErrInvalidURL)
	}
	parsedURL, err := url.ParseRequestURI(urlStr)
	if err != nil {
		return fmt.Errorf("invalid url format: %w", entity.ErrInvalidURL)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("url must use http or https scheme: %w", entity.ErrInvalidURL)
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("url must have a host: %w", entity.ErrInvalidURL)
	}

	return nil
}

func (s *ShortenerService) validateCustomAlias(alias string) error {
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
		return entity.ErrURLInactive
	}

	if url.ExpiresAt != nil && url.ExpiresAt.Before(time.Now()) {
		return entity.ErrURLExpired
	}

	return nil
}

func (s *ShortenerService) buildShortURL(shortCode string) string {
	return fmt.Sprintf("%s/%s", strings.TrimSuffix(s.baseURL, "/"), shortCode)
}

func (s *ShortenerService) recordClickAndIncrement(
	ctx context.Context,
	urlID uuid.UUID,
	shortCode string,
	clickInfo ClickInfo,
) error {
	const op = "service.recordClickAndIncrement"

	err := s.tm.ExecuteInTransaction(ctx, op, func(tx pgxdriver.QueryExecuter) error {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("generate id: %w", err)
		}

		analytics := entity.Analytics{
			ID:        id,
			URLID:     urlID,
			UserAgent: clickInfo.UserAgent,
			IPAddress: clickInfo.IPAddress,
			Referer:   clickInfo.Referer,
			ClickedAt: time.Now(),
		}

		if err = s.analyticsRepo.RecordClick(ctx, tx, analytics); err != nil {
			return fmt.Errorf("record click: %w", err)
		}

		if err = s.urlRepo.IncrementClickCount(ctx, tx, urlID); err != nil {
			return fmt.Errorf("increment click count: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	if err = s.cache.IncrementClickCount(ctx, shortCode); err != nil {
		if !errors.Is(err, entity.ErrDataNotFound) {
			s.log.Ctx(ctx).LogAttrs(ctx, logger.DebugLevel, "cache increment failed",
				logger.String("short_code", shortCode),
				logger.Any("error", err),
			)
		}
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
		s.log.LogAttrs(ctx, logger.WarnLevel, "slow operation detected", allAttrs...)
	}
}
