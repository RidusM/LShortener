package app

import (
	"context"
	"errors"
	"fmt"

	"lshortener/internal/config"
	"lshortener/internal/repository"
	"lshortener/internal/service"
	handler "lshortener/internal/transport/http"
	"lshortener/pkg/keygen"

	pgxdriver "github.com/wb-go/wbf/dbpg/pgx-driver"
	"github.com/wb-go/wbf/dbpg/pgx-driver/transaction"
	"github.com/wb-go/wbf/logger"
	"github.com/wb-go/wbf/redis"
	"golang.org/x/sync/errgroup"
)

func Run(ctx context.Context, cfg *config.Config, log logger.Logger) error {
	var (
		db  *pgxdriver.Postgres
		rdb *redis.Client
		tm  transaction.Manager
		svc *service.ShortenerService
		err error
	)

	defer func() {
		if rdb != nil {
			if err = rdb.Close(); err != nil {
				log.Error("failed to close Redis client", "error", err)
			} else {
				log.Info("Redis client closed")
			}
		}
		if db != nil {
			db.Close()
			log.Info("database connection pool closed")
		}
	}()
	db, err = initDatabase(&cfg.Database, log)
	if err != nil {
		return fmt.Errorf("init databse: %w", err)
	}
	log.Info("database initialized successfully")

	tm, err = initTransactionManager(db, log)
	if err != nil {
		return fmt.Errorf("init transaction manager: %w", err)
	}

	rdb = initCache(&cfg.Cache)
	log.Info("cache initialized successfully")

	svc, err = initShortenerService(&cfg.Service, db, tm, rdb, log)
	if err != nil {
		return fmt.Errorf("init shortener service: %w", err)
	}

	eg, ctx := errgroup.WithContext(ctx)

	initHTTPServer(ctx, eg, &cfg.HTTP, svc, log)
	if err = eg.Wait(); err != nil {
		if !errors.Is(err, context.Canceled) {
			log.Error("application shutdown with error", "error", err)
			return fmt.Errorf("application shutdown error: %w", err)
		}
	}

	log.Info("application shutdown complete")
	return nil
}

func initDatabase(cfg *config.Database, log logger.Logger) (*pgxdriver.Postgres, error) {
	db, err := pgxdriver.New(
		cfg.DSN,
		log,
		pgxdriver.MaxPoolSize(cfg.PoolMax),
		pgxdriver.MaxConnAttempts(cfg.ConnAttempts),
		pgxdriver.BaseRetryDelay(cfg.BaseRetryDelay),
		pgxdriver.MaxRetryDelay(cfg.MaxRetryDelay),
	)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	return db, nil
}

func initTransactionManager(db *pgxdriver.Postgres, log logger.Logger) (transaction.Manager, error) {
	tm, err := transaction.NewManager(db, log)
	if err != nil {
		return nil, fmt.Errorf("create transaction manager: %w", err)
	}
	return tm, nil
}

func initCache(cfg *config.Cache) *redis.Client {
	return redis.New(cfg.Addr, cfg.Password, 0)
}

func initShortenerService(
	cfg *config.Service,
	db *pgxdriver.Postgres,
	tm transaction.Manager,
	rdb *redis.Client,
	log logger.Logger,
) (*service.ShortenerService, error) {
	urlRepo := repository.NewURLRepository(db)
	analyticRepo := repository.NewAnalyticsRepository(db)
	cacheRepo := repository.NewCacheRepository(rdb)

	svc, err := service.NewShortenerService(
		urlRepo,
		analyticRepo,
		cacheRepo,
		tm,
		log,
		keygen.NewBase62Generator(),

		service.WithShortCodeLength(cfg.ShortCodeLength),
		service.WithDefaultTTL(cfg.DefaultTTL),
		service.WithBaseURL(cfg.BaseURL),
		service.WithMaxRetries(cfg.MaxRetries),
	)
	if err != nil {
		return nil, fmt.Errorf("create notify service: %w", err)
	}
	return svc, nil
}

func initHTTPServer(
	ctx context.Context,
	eg *errgroup.Group,
	cfg *config.HTTP,
	svc *service.ShortenerService,
	log logger.Logger,
) {
	sHandler := handler.NewShortenerHandler(svc, log)
	httpServer := handler.NewHTTPServer(sHandler, cfg, log)

	eg.Go(func() error {
		if err := httpServer.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("http server error: %w", err)
		}
		return nil
	})
}
