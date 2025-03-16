package storage

import (
	"context"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

const setSQLQuery = "INSERT INTO entries (key, value, expiration) VALUES ($1, $2, $3) ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, expiration = EXCLUDED.expiration"
const getSQLQuery = "SELECT id, value, expiration FROM entries WHERE key = $1"
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
		logrus.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}

	// Check if connection is established
	if err := pool.Ping(context.Background()); err != nil {
		logrus.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}

	// Create table if not exists
	_, err = pool.Exec(context.Background(), "CREATE TABLE IF NOT EXISTS entries (id SERIAL PRIMARY KEY, key VARCHAR(255) NOT NULL UNIQUE, value TEXT, expiration BIGINT)")
	if err != nil {
		logrus.Fatalf("Failed to create table: %v", err)
	}

	return &PostgresStorage{pool: pool, expiration: expiration}
}

func (s *PostgresStorage) Set(key string, value string, skip_expiration bool) error {
	ctx := context.Background() // TODO: Add timeout control

	now := time.Now()
	expiration := now.Add(time.Duration(s.expiration)).Unix()
	if skip_expiration {
		expiration = 0
	}

	_, err := s.pool.Exec(ctx, setSQLQuery, key, value, expiration)
	return err
}

func (s *PostgresStorage) Get(key string, skip_expiration bool) (string, error) {
	ctx := context.Background() // TODO: Add timeout control

	var id int
	var value string
	var expiration int64

	err := s.pool.QueryRow(ctx, getSQLQuery, key).Scan(&id, &value, &expiration)
	if err != nil {
		return "", err
	}

	// Delete if expired
	if expiration != 0 && time.Now().Unix() > expiration {
		_, err = s.pool.Exec(ctx, deleteSQLQuery, id)
		if err != nil {
			return "", err
		}
		return "", nil
	}

	// Update expiration
	if !skip_expiration {
		_, err = s.pool.Exec(ctx, updateSQLQuery, time.Now().Add(time.Duration(s.expiration)).Unix(), id)
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
