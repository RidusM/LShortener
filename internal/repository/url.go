package repository

import (
	"context"
	"errors"
	"fmt"

	"lshortener/internal/entity"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
	pgxdriver "github.com/wb-go/wbf/dbpg/pgx-driver"
)

type URLRepository struct {
	db *pgxdriver.Postgres
}

func NewURLRepository(db *pgxdriver.Postgres) *URLRepository {
	return &URLRepository{db: db}
}

func (r *URLRepository) Create(ctx context.Context, qe pgxdriver.QueryExecuter, url entity.URL) (*entity.URL, error) {
	const op = "repository.url.Create"

	executor := execOrDB(qe, r.db)

	var err error
	url.ID, err = uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("%s: v7 uuid: %w", op, err)
	}
	url.IsActive = true
	url.ClickCount = 0

	insert := r.db.Insert("urls").
		Columns("id", "short_code", "original_url", "custom_alias", "expires_at", "is_active", "click_count").
		Values(url.ID, url.ShortCode, url.OriginalURL, url.CustomAlias, url.ExpiresAt, url.IsActive, url.ClickCount).
		Suffix("RETURNING id, short_code, original_url, custom_alias, expires_at, is_active, click_count")

	query, args, err := insert.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: insert query: %w", op, err)
	}

	var result entity.URL
	err = executor.QueryRow(ctx, query, args...).Scan(
		&result.ID,
		&result.ShortCode,
		&result.OriginalURL,
		&result.CustomAlias,
		&result.ExpiresAt,
		&result.IsActive,
		&result.ClickCount,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &result, nil
}

func (r *URLRepository) GetByShortCode(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	shortCode string,
) (*entity.URL, error) {
	const op = "repository.url.GetByShortCode"
	return r.getURLByField(ctx, qe, "short_code", shortCode, op)
}

func (r *URLRepository) GetByCustomAlias(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	alias string,
) (*entity.URL, error) {
	const op = "repository.url.GetByCustomAlias"
	return r.getURLByField(ctx, qe, "custom_alias", alias, op)
}

func (r *URLRepository) IncrementClickCount(ctx context.Context, qe pgxdriver.QueryExecuter, urlID uuid.UUID) error {
	const op = "repository.url.IncrementClickCount"

	executor := execOrDB(qe, r.db)

	update := r.db.Update("urls").
		Set("click_count", squirrel.Expr("click_count + 1")).
		Where(squirrel.Eq{"id": urlID})

	query, args, err := update.ToSql()
	if err != nil {
		return fmt.Errorf("%s: update query: %w", op, err)
	}

	_, err = executor.Exec(ctx, query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("%s: %w", op, entity.ErrConflictingData)
		}
		return fmt.Errorf("%s: exec: %w", op, err)
	}

	return nil
}

func (r *URLRepository) ShortCodeExists(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	shortCode string,
) (bool, error) {
	const op = "repository.ulr.ShortCodeExists"

	executor := execOrDB(qe, r.db)

	selectQuery := r.db.Select("EXISTS(SELECT 1 FROM urls WHERE short_code = ?)", shortCode)

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return false, fmt.Errorf("%s: select query: %w", op, err)
	}

	var exists bool
	err = executor.QueryRow(ctx, query, args...).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("%s: %w", op, err)
	}

	return exists, nil
}

func (r *URLRepository) CustomAliasExists(ctx context.Context, qe pgxdriver.QueryExecuter, alias string) (bool, error) {
	const op = "repository.url.CustomAliasExists"

	executor := execOrDB(qe, r.db)

	selectQuery := r.db.Select("EXISTS(SELECT 1 FROM urls WHERE custom_alias = ?)", alias)

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return false, fmt.Errorf("%s: select query: %w", op, err)
	}

	var exists bool
	err = executor.QueryRow(ctx, query, args...).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("%s: %w", op, err)
	}

	return exists, nil
}

func (r *URLRepository) getURLByField(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	field string,
	value any,
	op string,
) (*entity.URL, error) {
	executor := execOrDB(qe, r.db)
	selectQuery := r.db.Select(
		"id", "short_code", "original_url", "custom_alias",
		"expires_at", "is_active", "click_count",
	).From("urls").Where(squirrel.Eq{field: value})

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: build query: %w", op, err)
	}

	var url entity.URL
	if qrErr := executor.QueryRow(ctx, query, args...).Scan(
		&url.ID, &url.ShortCode, &url.OriginalURL, &url.CustomAlias,
		&url.ExpiresAt, &url.IsActive, &url.ClickCount,
	); qrErr != nil {
		if errors.Is(qrErr, pgx.ErrNoRows) {
			return nil, entity.ErrDataNotFound
		}
		return nil, fmt.Errorf("%s: %w", op, qrErr)
	}
	return &url, nil
}
