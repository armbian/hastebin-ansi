package storage

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/armbian/ansi-hastebin/internal/unsafeconv"
	"github.com/klauspost/compress/zstd"
)

type FileStorage struct {
	path        string
	compression string
}

var _ Storage = (*FileStorage)(nil)

func md5Hex(input string) string {
	sum := md5.Sum([]byte(input))
	return hex.EncodeToString(sum[:])
}

func NewFileStorage(path, compression string, _ time.Duration) Storage {
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		os.Mkdir(path, 0700)
	}

	return &FileStorage{path: path, compression: compression}
}

func (fs *FileStorage) Set(key string, value string, skip_expiration bool) error {
	dst := filepath.Join(fs.path, md5Hex(key))

	var output []byte
	var err error
	var suffix string

	switch fs.compression {
	case "zstd":
		suffix = ".zst"
		var buf bytes.Buffer
		w, _ := zstd.NewWriter(&buf)
		w.Write(unsafeconv.UnsafeBytes(value))
		w.Close()
		output = buf.Bytes()
	case "gzip":
		suffix = ".gz"
		var buf bytes.Buffer
		w := gzip.NewWriter(&buf)
		w.Write(unsafeconv.UnsafeBytes(value))
		w.Close()
		output = buf.Bytes()
	default:
		suffix = ""
		output = unsafeconv.UnsafeBytes(value)
	}

	dst += suffix

	file, err := os.CreateTemp(fs.path, ".hastebin-*")
	if err != nil {
		return err
	}
	tempName := file.Name()
	defer os.Remove(tempName)

	if _, err = file.Write(output); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = os.Rename(tempName, dst); err != nil {
		return err
	}

	baseDst := filepath.Join(fs.path, md5Hex(key))
	for _, alternative := range []string{"", ".gz", ".zst"} {
		if baseDst+alternative != dst {
			if err := os.Remove(baseDst + alternative); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}

	return nil
}

func (fs *FileStorage) Get(key string, skip_expiration bool) (string, error) {
	baseDst := filepath.Join(fs.path, md5Hex(key))

	// According to requirements: if the file found first uncompressed use the uncompressed one.
	if data, err := os.ReadFile(baseDst); err == nil {
		return unsafeconv.UnsafeString(data), nil
	}

	// Try zstd
	if data, err := os.ReadFile(baseDst + ".zst"); err == nil {
		r, _ := zstd.NewReader(bytes.NewReader(data))
		defer r.Close()
		decompressed, err := io.ReadAll(r)
		if err == nil {
			return unsafeconv.UnsafeString(decompressed), nil
		}
	}

	// Try gzip
	if data, err := os.ReadFile(baseDst + ".gz"); err == nil {
		r, err := gzip.NewReader(bytes.NewReader(data))
		if err == nil {
			defer r.Close()
			decompressed, err := io.ReadAll(r)
			if err == nil {
				return unsafeconv.UnsafeString(decompressed), nil
			}
		}
	}

	return "", os.ErrNotExist
}

func (fs *FileStorage) Close() error {
	return nil
}
