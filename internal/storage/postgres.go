package storage

import (
	"context"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

const setSQLQuery = "INSERT INTO entries (key, value, expiration, fixed_expiration) VALUES ($1, $2, $3, $4) ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, expiration = EXCLUDED.expiration, fixed_expiration = EXCLUDED.fixed_expiration"
const getSQLQuery = "SELECT id, value, expiration, fixed_expiration FROM entries WHERE key = $1"
const deleteSQLQuery = "DELETE FROM entries WHERE id = $1"
const updateSQLQuery = "UPDATE entries SET expiration = $1 WHERE id = $2"

type PostgresStorage struct {
	pool       *pgxpool.Pool
	expiration int
}

var _ Storage = (*PostgresStorage)(nil)

func NewPostgresStorage(host string, port int, username string, passowrd string, database string, expiration int) *PostgresStorage {
	dsn := "postgres://" + username + ":" + passowrd + "@" + host + ":" + strconv.Itoa(port) + "/" + database
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to PostgreSQL")
	}

	// Check if connection is established
	if err := pool.Ping(context.Background()); err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to PostgreSQL")
	}

	// Create table if not exists
	_, err = pool.Exec(context.Background(), "CREATE TABLE IF NOT EXISTS entries (id SERIAL PRIMARY KEY, key VARCHAR(255) NOT NULL UNIQUE, value TEXT, expiration BIGINT, fixed_expiration BOOLEAN NOT NULL DEFAULT FALSE)")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create table")
	}
	_, err = pool.Exec(context.Background(), "ALTER TABLE entries ADD COLUMN IF NOT EXISTS fixed_expiration BOOLEAN NOT NULL DEFAULT FALSE")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to migrate entries table")
	}

	return &PostgresStorage{pool: pool, expiration: expiration}
}

func (s *PostgresStorage) Set(key string, value string, skip_expiration bool) error {
	ctx := context.Background() // TODO: Add timeout control

	now := time.Now()
	expiration := now.Add(time.Duration(s.expiration) * time.Second).Unix()
	if skip_expiration {
		expiration = 0
	}

	_, err := s.pool.Exec(ctx, setSQLQuery, key, value, expiration, false)
	return err
}

func (s *PostgresStorage) SetWithDeleteAfter(key, value string, deleteAfter time.Duration) error {
	_, err := s.pool.Exec(context.Background(), setSQLQuery, key, value, time.Now().Add(deleteAfter).Unix(), true)
	return err
}

func (s *PostgresStorage) Get(key string, skip_expiration bool) (string, error) {
	ctx := context.Background() // TODO: Add timeout control

	var id int
	var value string
	var expiration int64
	var fixedExpiration bool

	err := s.pool.QueryRow(ctx, getSQLQuery, key).Scan(&id, &value, &expiration, &fixedExpiration)
	if err != nil {
		return "", err
	}

	// Delete if expired
	if expiration != 0 && time.Now().Unix() >= expiration {
		_, err = s.pool.Exec(ctx, deleteSQLQuery, id)
		if err != nil {
			return "", err
		}
		return "", nil
	}

	// Update expiration
	if !skip_expiration && !fixedExpiration {
		_, err = s.pool.Exec(ctx, updateSQLQuery, time.Now().Add(time.Duration(s.expiration)*time.Second).Unix(), id)
		if err != nil {
			return "", err
		}
	}

	return value, nil
}

func (s *PostgresStorage) Close() error {
	s.pool.Close()
	return nil
}
