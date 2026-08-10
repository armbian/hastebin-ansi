package storage

import "time"

type Storage interface {
	Set(key string, value string, skip_expiration bool) error
	Get(key string, skip_expiration bool) (string, error)
	Close() error
}

// DeleteAfterStorage persists a paste with a fixed lifetime. Implementations
// must not extend this deadline when the paste is read.
type DeleteAfterStorage interface {
	SetWithDeleteAfter(key, value string, deleteAfter time.Duration) error
}

type ExpiredPasteCleaner interface {
	CleanupExpired() (int, error)
}
