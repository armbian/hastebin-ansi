package storage

import (
	"strconv"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/sirupsen/logrus"
)

type MemcachedStorage struct {
	client     *memcache.Client
	expiration int
}

func NewMemcachedStorage(host string, port int, expiration int) *MemcachedStorage {
	client := memcache.New(host + ":" + strconv.Itoa(port))

	// Check if connection is established
	if err := client.Ping(); err != nil {
		logrus.Fatalf("Failed to connect to Memcached: %v", err)
	}

	return &MemcachedStorage{client: client, expiration: expiration}
}

var _ Storage = (*MemcachedStorage)(nil)

func (s *MemcachedStorage) Set(key string, value string, skip_expiration bool) error {
	item := &memcache.Item{
		Key:        key,
		Value:      []byte(value),
		Expiration: int32(s.expiration),
	}
	if skip_expiration {
		item.Expiration = 0
	}
	return s.client.Set(item)
}

func (s *MemcachedStorage) Get(key string, skip_expiration bool) (string, error) {
	item, err := s.client.Get(key)
	if err != nil {
		return "", err
	}

	if !skip_expiration {
		s.client.Replace(&memcache.Item{
			Key:        key,
			Value:      item.Value,
			Expiration: int32(s.expiration),
		})
	}

	return string(item.Value), nil
}

func (s *MemcachedStorage) Close() error {
	return s.client.Close()
}
