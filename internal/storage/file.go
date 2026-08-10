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
	"strconv"
	"strings"
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
	return fs.set(key, value, 0)
}

func (fs *FileStorage) SetWithDeleteAfter(key, value string, deleteAfter time.Duration) error {
	return fs.set(key, value, deleteAfter)
}

func (fs *FileStorage) set(key string, value string, deleteAfter time.Duration) error {
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

	metadataPath := baseDst + ".expires"
	if deleteAfter <= 0 {
		if err := os.Remove(metadataPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return os.WriteFile(metadataPath, []byte(strconv.FormatInt(time.Now().Add(deleteAfter).Unix(), 10)), 0600)
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

func (fs *FileStorage) CleanupExpired() (int, error) {
	dir, err := os.Open(fs.path)
	if err != nil {
		return 0, err
	}
	defer dir.Close()

	removed := 0
	for {
		names, readErr := dir.Readdirnames(512)
		for _, name := range names {
			if !strings.HasSuffix(name, ".expires") {
				continue
			}
			base := filepath.Join(fs.path, strings.TrimSuffix(name, ".expires"))
			data, err := os.ReadFile(filepath.Join(fs.path, name))
			if err != nil {
				return removed, err
			}
			expiresAt, err := strconv.ParseInt(string(data), 10, 64)
			if err != nil {
				return removed, err
			}
			if time.Now().Unix() < expiresAt {
				continue
			}
			for _, suffix := range []string{"", ".gz", ".zst", ".expires"} {
				if err := os.Remove(base + suffix); err != nil && !os.IsNotExist(err) {
					return removed, err
				}
			}
			removed++
		}
		if readErr == io.EOF {
			return removed, nil
		}
		if readErr != nil {
			return removed, readErr
		}
	}
}
