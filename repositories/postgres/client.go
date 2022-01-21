package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/cobbinma/track-api/graph/model"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	"net/url"
)

type Client struct {
	db *sqlx.DB
}

type journey struct {
	ID     string          `db:"id"`
	UserId string          `db:"user_id"`
	Status string          `db:"status"`
	Lat    sql.NullFloat64 `db:"lat"`
	Lng    sql.NullFloat64 `db:"lng"`
}

func (j journey) Position() *model.Position {
	if j.Lat.Valid && j.Lng.Valid {
		return &model.Position{
			Lat: j.Lat.Float64,
			Lng: j.Lng.Float64,
		}
	}

	return nil
}

func NewPostgres(url url.URL, migrationsUrl string) (*Client, error) {
	db, err := sqlx.Connect("postgres", url.String())
	if err != nil {
		return nil, err
	}

	driver, err := postgres.WithInstance(db.DB, &postgres.Config{})
	m, err := migrate.NewWithDatabaseInstance(
		migrationsUrl,
		"postgres", driver)
	if err != nil {
		return nil, err
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return nil, err
	}

	return &Client{db: db}, nil
}

func (c Client) GetJourney(ctx context.Context, id string) (*model.Journey, error) {
	query, args, err := sq.
		Select("id", "user_id", "status", "lat", "lng").
		From("journeys").
		Where(sq.Eq{"id": id}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("to sql : %w", err)
	}

	var j = &journey{}
	if err := c.db.GetContext(ctx, j, query, args...); err != nil {
		return nil, fmt.Errorf("get : %w", err)
	}

	return &model.Journey{
		ID:       j.ID,
		User:     &model.User{ID: j.UserId},
		Status:   model.JourneyStatus(j.Status),
		Position: j.Position(),
	}, nil
}

func (c Client) CreateJourney(ctx context.Context, journey *model.Journey) error {
	query, args, err := sq.
		Insert("journeys").
		Columns("id", "user_id", "status").
		Values(journey.ID, journey.User.ID, journey.Status).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("to sql : %w", err)
	}

	if _, err := c.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("exec context : %w", err)
	}

	return nil
}

func (c Client) UpdatePosition(ctx context.Context, id string, position *model.Position) error {
	var lat, lng sql.NullFloat64
	if position != nil {
		lat = sql.NullFloat64{
			Float64: position.Lat,
			Valid:   true,
		}
		lng = sql.NullFloat64{
			Float64: position.Lng,
			Valid:   true,
		}
	}
	query, args, err := sq.
		Update("journeys").
		Set("lat", lat).
		Set("lng", lng).
		Where(sq.Eq{"id": id}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("to sql : %w", err)
	}

	if _, err := c.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("exec context : %w", err)
	}

	return nil
}

func (c Client) UpdateStatus(ctx context.Context, id string, status model.JourneyStatus) error {
	query, args, err := sq.
		Update("journeys").
		Set("status", status).
		Where(sq.Eq{"id": id}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("to sql : %w", err)
	}

	if _, err := c.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("exec context : %w", err)
	}

	return nil
}
