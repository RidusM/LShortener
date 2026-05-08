//nolint:musttag
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"lshortener/internal/entity"

	"github.com/go-redis/redis/v8"
	rediswbf "github.com/wb-go/wbf/redis"
)

const (
	_cacheKeyPrefix = "short_link:"
	_defaultTTL     = 5 * time.Minute
)

type CacheRepository struct {
	rdb *rediswbf.Client
}

func NewCacheRepository(rdb *rediswbf.Client) *CacheRepository {
	return &CacheRepository{rdb: rdb}
}

func (r *CacheRepository) cacheKey(shortCode string) string {
	return _cacheKeyPrefix + shortCode
}

func (r *CacheRepository) Get(ctx context.Context, shortCode string) (*entity.URL, error) {
	const op = "repository.cache.Get"

	cached, err := r.rdb.Get(ctx, r.cacheKey(shortCode))
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, entity.ErrDataNotFound
		}
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	if cached == "" {
		return nil, entity.ErrDataNotFound
	}

	var url entity.URL
	if unmarshErr := json.Unmarshal([]byte(cached), &url); unmarshErr != nil {
		return nil, fmt.Errorf("%s: unmarshal: %w", op, unmarshErr)
	}

	return &url, nil
}

func (r *CacheRepository) Save(ctx context.Context, url *entity.URL) error {
	const op = "repository.cache.Save"

	data, err := json.Marshal(url)
	if err != nil {
		return fmt.Errorf("%s: marshal: %w", op, err)
	}

	if err = r.rdb.SetWithExpiration(ctx, r.cacheKey(url.ShortCode), data, _defaultTTL); err != nil {
		return fmt.Errorf("%s: redis set: %w", op, err)
	}
	return nil
}

func (r *CacheRepository) Invalidate(ctx context.Context, shortCode string) error {
	const op = "repository.cache.Invalidate"

	if err := r.rdb.Del(ctx, r.cacheKey(shortCode)); err != nil {
		if errors.Is(err, redis.Nil) {
			return nil
		}
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (r *CacheRepository) IncrementClickCount(ctx context.Context, shortCode string) error {
	const op = "repository.cache.IncrementClickCount"

	url, err := r.Get(ctx, r.cacheKey(shortCode))
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	if err = r.rdb.Incr(ctx, url.ShortCode).Err(); err != nil {
		return fmt.Errorf("%s: redis incr: %w", op, err)
	}

	return nil
}
