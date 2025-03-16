package storage

type Storage interface {
	Set(key string, value string, skip_expiration bool) error
	Get(key string, skip_expiration bool) (string, error)
	Close() error
}
