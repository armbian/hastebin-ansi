package main

import (
	"bytes"
	"compress/gzip"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/klauspost/compress/zstd"
)

func main() {
	dirPtr := flag.String("dir", "data", "Directory containing pastes")
	compPtr := flag.String("compression", "zstd", "Compression format (zstd or gzip)")
	flag.Parse()

	dir := *dirPtr
	format := *compPtr

	if format != "zstd" && format != "gzip" {
		log.Fatalf("Unsupported compression format: %s", format)
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		log.Fatalf("Failed to read directory: %v", err)
	}

	count := 0
	for _, f := range files {
		// Skip directories and files that already have an extension
		if f.IsDir() || filepath.Ext(f.Name()) != "" {
			continue
		}

		// MD5 hex strings are exactly 32 chars
		if len(f.Name()) != 32 {
			continue
		}

		oldPath := filepath.Join(dir, f.Name())
		data, err := os.ReadFile(oldPath)
		if err != nil {
			log.Printf("Failed to read %s: %v", oldPath, err)
			continue
		}

		var newPath string
		var output []byte

		switch format {
		case "zstd":
			newPath = oldPath + ".zst"
			var buf bytes.Buffer
			w, _ := zstd.NewWriter(&buf)
			w.Write(data)
			w.Close()
			output = buf.Bytes()
		case "gzip":
			newPath = oldPath + ".gz"
			var buf bytes.Buffer
			w := gzip.NewWriter(&buf)
			w.Write(data)
			w.Close()
			output = buf.Bytes()
		}

		err = os.WriteFile(newPath, output, 0644)
		if err != nil {
			log.Printf("Failed to write %s: %v", newPath, err)
			continue
		}

		// Remove old uncompressed file to save space
		err = os.Remove(oldPath)
		if err != nil {
			log.Printf("Failed to remove old file %s: %v", oldPath, err)
		}

		count++
		fmt.Printf("Compressed %s -> %s\n", f.Name(), filepath.Base(newPath))
	}

	fmt.Printf("Successfully compressed %d files.\n", count)
}
