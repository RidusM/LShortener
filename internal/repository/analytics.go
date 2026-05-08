package repository

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"lshortener/internal/entity"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	pgxdriver "github.com/wb-go/wbf/dbpg/pgx-driver"
)

const (
	_analyticsColumns = "id, url_id, user_agent, ip_address, referer, clicked_at"
	columnURLID       = "url_id"
)

type AnalyticsRepository struct {
	db *pgxdriver.Postgres
}

func NewAnalyticsRepository(db *pgxdriver.Postgres) *AnalyticsRepository {
	return &AnalyticsRepository{db: db}
}

func (r *AnalyticsRepository) RecordClick(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	a entity.Analytics,
) error {
	const op = "repository.analytics.RecordClick"

	sql, args, err := r.db.Insert("analytics").
		Columns(_analyticsColumns).
		Values(a.ID, a.URLID, a.UserAgent, a.IPAddress, a.Referer, a.ClickedAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	_, err = execOrDB(qe, r.db).Exec(ctx, sql, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("%s: %w", op, entity.ErrConflictingData)
		}
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (r *AnalyticsRepository) GetURLInfoByShortCode(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	shortCode string,
) (*entity.URL, error) {
	const op = "repository.analytics.GetURLInfoByShortCode"

	sql, args, err := r.db.Select("id", "short_code", "original_url", "is_active", "expires_at", "click_count", "created_at").
		From("urls").
		Where(squirrel.Eq{"short_code": shortCode}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	var url entity.URL
	err = qe.QueryRow(ctx, sql, args...).Scan(
		&url.ID,
		&url.ShortCode,
		&url.OriginalURL,
		&url.IsActive,
		&url.ExpiresAt,
		&url.ClickCount,
		&url.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, entity.ErrDataNotFound
		}
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &url, nil
}

func (r *AnalyticsRepository) GetClicksByDay(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	urlID uuid.UUID,
	limit uint64,
) (map[string]int64, error) {
	const op = "repository.analytics.GetClicksByDay"

	query := r.db.Select(
		"DATE_TRUNC('day', clicked_at) as day",
		"COUNT(*) as clicks",
	).
		From("analytics").
		Where(squirrel.Eq{columnURLID: urlID}).
		GroupBy("day").
		OrderBy("day DESC").
		Limit(limit)

	return r.scanStatsMap(ctx, qe, query, op, true)
}

func (r *AnalyticsRepository) GetClicksByUA(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	urlID uuid.UUID,
	limit uint64,
) (map[string]int64, error) {
	const op = "repository.analytics.GetClicksByUA"

	query := r.db.Select("user_agent", "COUNT(*) as count").
		From("analytics").
		Where(squirrel.Eq{columnURLID: urlID}).
		GroupBy("user_agent").
		OrderBy("count DESC").
		Limit(limit)

	return r.scanStatsMap(ctx, qe, query, op, false)
}

func (r *AnalyticsRepository) GetRecentClicks(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	urlID uuid.UUID,
	limit uint64,
) ([]entity.Analytics, error) {
	const op = "repository.analytics.GetRecentClicks"

	query := r.db.Select(_analyticsColumns).
		From("analytics").
		Where(squirrel.Eq{columnURLID: urlID}).
		OrderBy("clicked_at DESC").
		Limit(limit)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	rows, err := qe.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	var clicks []entity.Analytics
	for rows.Next() {
		var c entity.Analytics
		var ip net.IP

		if err = rows.Scan(
			&c.ID,
			&c.URLID,
			&c.UserAgent,
			&ip,
			&c.Referer,
			&c.ClickedAt); err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}

		c.IPAddress = ip.String()
		clicks = append(clicks, c)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return clicks, nil
}

func (r *AnalyticsRepository) scanStatsMap(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	builder squirrel.SelectBuilder,
	op string,
	isDate bool,
) (map[string]int64, error) {
	sql, args, err := builder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: to sql: %w", op, err)
	}

	rows, err := qe.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: query: %w", op, err)
	}
	defer rows.Close()

	result := make(map[string]int64)
	for rows.Next() {
		var count int64
		var key string

		if isDate {
			var t time.Time
			if err = rows.Scan(&t, &count); err != nil {
				return nil, fmt.Errorf("%s: scan date: %w", op, err)
			}
			key = t.Format("2006-01-02")
		} else {
			if err = rows.Scan(&key, &count); err != nil {
				return nil, fmt.Errorf("%s: scan key: %w", op, err)
			}
		}
		result[key] = count
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows error: %w", op, err)
	}

	return result, nil
}
