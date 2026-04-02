package repository

import (
	"context"
	"fmt"
	"time"

	"lshortener/internal/entity"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	pgxdriver "github.com/wb-go/wbf/dbpg/pgx-driver"
)

const (
	_recentClicksLimit = 30
	_analyticsLimit    = 10
)

type AnalyticsRepository struct {
	db *pgxdriver.Postgres
}

type urlShortInfo struct {
	ShortCode   string
	OriginalURL string
	TotalClicks int64
	CreatedAt   time.Time
}

func NewAnalyticsRepository(db *pgxdriver.Postgres) *AnalyticsRepository {
	return &AnalyticsRepository{db: db}
}

func (r *AnalyticsRepository) RecordClick(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	analytics entity.Analytics,
) error {
	const op = "repository.analytics.RecordClick"

	executor := execOrDB(qe, r.db)

	if analytics.ID == uuid.Nil {
		var err error
		analytics.ID, err = uuid.NewV7()
		if err != nil {
			return fmt.Errorf("%s: v7 uuid: %w", op, err)
		}
	}

	insert := r.db.Insert("analytics").
		Columns("id", "url_id", "user_agent", "ip_address", "referer").
		Values(analytics.ID, analytics.URLId, analytics.UserAgent, analytics.IPAddress, analytics.Referer)

	query, args, err := insert.ToSql()
	if err != nil {
		return fmt.Errorf("%s: insert query: %w", op, err)
	}

	_, err = executor.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (r *AnalyticsRepository) GetStatsByShortCode(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	shortCode string,
	limit uint64,
) (*entity.AnalyticsStats, error) {
	const op = "repository.analytics.GetStatsByShortCode"

	urlInfo, urlID, err := r.fetchURLInfo(ctx, qe, shortCode, op)
	if err != nil {
		return nil, err
	}

	clicksByDay, err := r.fetchClicksByDay(ctx, qe, urlID, op)
	if err != nil {
		return nil, err
	}

	clicksByUA, err := r.fetchClicksByUA(ctx, qe, urlID, op)
	if err != nil {
		return nil, err
	}

	recentClicks, err := r.fetchRecentClicks(ctx, qe, urlID, limit, op)
	if err != nil {
		return nil, err
	}

	return &entity.AnalyticsStats{
		ShortCode:    urlInfo.ShortCode,
		OriginalURL:  urlInfo.OriginalURL,
		TotalClicks:  urlInfo.TotalClicks,
		CreatedAt:    urlInfo.CreatedAt,
		ClicksByDay:  clicksByDay,
		ClicksByUA:   clicksByUA,
		RecentClicks: recentClicks,
	}, nil
}

func (r *AnalyticsRepository) fetchURLInfo(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	shortCode, op string,
) (*urlShortInfo, uuid.UUID, error) {
	executor := execOrDB(qe, r.db)

	urlSelect := r.db.Select("u.id", "u.short_code", "u.original_url", "u.click_count").
		From("urls u").
		Where(squirrel.Eq{"u.short_code": shortCode})

	sql, args, err := urlSelect.ToSql()
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("%s: url select: %w", op, err)
	}

	var info urlShortInfo
	var urlID uuid.UUID
	err = executor.QueryRow(ctx, sql, args...).Scan(
		&urlID,
		&info.ShortCode,
		&info.OriginalURL,
		&info.TotalClicks,
	)
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("%s: url info: %w", op, err)
	}

	info.CreatedAt = entity.ExtractTimestampFromUUIDv7(urlID)
	return &info, urlID, nil
}

func (r *AnalyticsRepository) fetchClicksByDay(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	urlID uuid.UUID,
	op string,
) (map[string]int64, error) {
	query := r.db.Select(
		"DATE_TRUNC('day', a.clicked_at) as day",
		"COUNT(*) as clicks",
	).
		From("analytics a").
		Where(squirrel.Eq{"a.url_id": urlID}).
		GroupBy("day").
		OrderBy("day DESC").
		Limit(_recentClicksLimit)

	return r.scanCountMap(ctx, qe, query, op, "day stats")
}

func (r *AnalyticsRepository) fetchClicksByUA(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	urlID uuid.UUID,
	op string,
) (map[string]int64, error) {
	query := r.db.Select("a.user_agent", "COUNT(*) as count").
		From("analytics a").
		Where(squirrel.Eq{"a.url_id": urlID}).
		GroupBy("a.user_agent").
		OrderBy("count DESC").
		Limit(_analyticsLimit)

	return r.scanCountMap(ctx, qe, query, op, "ua stats")
}

func (r *AnalyticsRepository) scanCountMap(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	builder squirrel.SelectBuilder,
	op, label string,
) (map[string]int64, error) {
	executor := execOrDB(qe, r.db)

	sql, args, err := builder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: %s query: %w", op, label, err)
	}

	rows, err := executor.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %s: %w", op, label, err)
	}
	defer rows.Close()

	result := make(map[string]int64)
	for rows.Next() {
		var key string
		var count int64

		var day time.Time
		if rowErr := rows.Scan(&key, &count); rowErr != nil {
			return nil, fmt.Errorf("%s: %s row: %w", op, label, rowErr)
		}
		key = day.Format("2006-01-02")

		result[key] = count
	}
	return result, nil
}

func (r *AnalyticsRepository) fetchRecentClicks(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	urlID uuid.UUID,
	limit uint64,
	op string,
) ([]entity.Analytics, error) {
	executor := execOrDB(qe, r.db)

	query := r.db.Select("id", "url_id", "user_agent", "ip_address", "referer", "clicked_at").
		From("analytics").
		Where(squirrel.Eq{"url_id": urlID}).
		OrderBy("clicked_at DESC").
		Limit(limit)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: recent clicks query: %w", op, err)
	}

	rows, err := executor.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: recent clicks: %w", op, err)
	}
	defer rows.Close()

	var clicks []entity.Analytics
	for rows.Next() {
		var c entity.Analytics
		if rowErr := rows.Scan(&c.ID, &c.URLId, &c.UserAgent, &c.IPAddress, &c.Referer, &c.ClickedAt); rowErr != nil {
			return nil, fmt.Errorf("%s: recent row: %w", op, rowErr)
		}
		clicks = append(clicks, c)
	}
	return clicks, nil
}
