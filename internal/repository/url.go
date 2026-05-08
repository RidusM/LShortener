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

const (
	_urlColumns = "id, short_code, original_url, custom_alias, expires_at, is_active, click_count, created_at"
)

type URLRepository struct {
	db *pgxdriver.Postgres
}

func NewURLRepository(db *pgxdriver.Postgres) *URLRepository {
	return &URLRepository{db: db}
}

func (r *URLRepository) Create(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	u entity.URL,
) (*entity.URL, error) {
	const op = "repository.url.Create"

	sql, args, err := r.db.Insert("urls").
		Columns("id", "short_code", "original_url", "custom_alias", "expires_at").
		Values(u.ID, u.ShortCode, u.OriginalURL, u.CustomAlias, u.ExpiresAt).
		Suffix("RETURNING " + _urlColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	var url entity.URL
	err = execOrDB(qe, r.db).QueryRow(ctx, sql, args...).Scan(
		&url.ID,
		&url.ShortCode,
		&url.OriginalURL,
		&url.CustomAlias,
		&url.ExpiresAt,
		&url.IsActive,
		&url.ClickCount,
		&url.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &url, nil
}

func (r *URLRepository) GetByShortCode(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	shortCode string,
) (*entity.URL, error) {
	const op = "repository.url.GetByShortCode"

	url, err := r.getURLByField(ctx, qe, "short_code", shortCode, op)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return url, nil
}

func (r *URLRepository) GetByCustomAlias(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	alias string,
) (*entity.URL, error) {
	const op = "repository.url.GetByCustomAlias"

	url, err := r.getURLByField(ctx, qe, "custom_alias", alias, op)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return url, nil
}

func (r *URLRepository) IncrementClickCount(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	urlID uuid.UUID,
) error {
	const op = "repository.url.IncrementClickCount"

	sql, args, err := r.db.Update("urls").
		Set("click_count", squirrel.Expr("click_count + 1")).
		Where(squirrel.Eq{"id": urlID}).
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

func (r *URLRepository) ShortCodeExists(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	shortCode string,
) (bool, error) {
	const op = "repository.url.ShortCodeExists"

	sql, args, err := r.db.Select("1").
		From("urls").
		Where(squirrel.Eq{"short_code": shortCode}).
		Prefix("SELECT EXISTS (").
		Suffix(")").
		ToSql()
	if err != nil {
		return false, fmt.Errorf("%s: %w", op, err)
	}

	var exists bool
	if err = execOrDB(qe, r.db).QueryRow(ctx, sql, args...).Scan(&exists); err != nil {
		return false, fmt.Errorf("%s: %w", op, err)
	}

	return exists, nil
}

func (r *URLRepository) CustomAliasExists(
	ctx context.Context,
	qe pgxdriver.QueryExecuter,
	alias string,
) (bool, error) {
	const op = "repository.url.CustomAliasExists"

	sql, args, err := r.db.Select("1").
		From("urls").
		Where(squirrel.Eq{"custom_alias": alias}).
		Prefix("SELECT EXISTS (").
		Suffix(")").
		ToSql()
	if err != nil {
		return false, fmt.Errorf("%s: %w", op, err)
	}

	var exists bool
	if err = execOrDB(qe, r.db).QueryRow(ctx, sql, args...).Scan(&exists); err != nil {
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
	sql, args, err := r.db.Select(_urlColumns).
		From("urls").
		Where(squirrel.Eq{field: value}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	var url entity.URL
	if err = execOrDB(qe, r.db).QueryRow(ctx, sql, args...).Scan(
		&url.ID,
		&url.ShortCode,
		&url.OriginalURL,
		&url.CustomAlias,
		&url.ExpiresAt,
		&url.IsActive,
		&url.ClickCount,
		&url.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, entity.ErrDataNotFound
		}
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return &url, nil
}
