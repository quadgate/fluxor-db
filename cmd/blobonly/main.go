package main

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type BlobMetadata struct {
	ContentType string            `json:"content_type"`
	Filename    string            `json:"filename,omitempty"`
	Size        int64             `json:"size"`
	Checksum    string            `json:"checksum"`
	Tags        map[string]string `json:"tags,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type BlobData struct {
	Key      string       `json:"key"`
	Data     []byte       `json:"-"`
	Metadata BlobMetadata `json:"metadata"`
}

type BlobInfo struct {
	Key      string       `json:"key"`
	Metadata BlobMetadata `json:"metadata"`
}

type BlobStats struct {
	TotalBlobs int64 `json:"total_blobs"`
	TotalSize  int64 `json:"total_size"`
	UsedSpace  int64 `json:"used_space"`
}

type FilesystemBlobStorage struct {
	rootPath string
	maxSize  int64
}

func NewFilesystemBlobStorage(root string, maxSize int64) (*FilesystemBlobStorage, error) {
	if root == "" {
		return nil, errors.New("-root is required")
	}
	if maxSize <= 0 {
		maxSize = 100 * 1024 * 1024
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create root: %w", err)
	}
	return &FilesystemBlobStorage{rootPath: root, maxSize: maxSize}, nil
}

func (fbs *FilesystemBlobStorage) Store(key string, data []byte, meta BlobMetadata) error {
	if len(data) > int(fbs.maxSize) {
		return fmt.Errorf("blob size %d exceeds maximum %d", len(data), fbs.maxSize)
	}
	filePath := filepath.Join(fbs.rootPath, filepath.Clean(key))
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fmt.Errorf("write blob: %w", err)
	}
	meta.Size = int64(len(data))
	meta.Checksum = fmt.Sprintf("%x", md5.Sum(data))
	if meta.CreatedAt.IsZero() {
		meta.CreatedAt = time.Now()
	}
	meta.UpdatedAt = time.Now()
	metaPath := filePath + ".meta"
	buf, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	return os.WriteFile(metaPath, buf, 0o644)
}

func (fbs *FilesystemBlobStorage) Retrieve(key string) (*BlobData, error) {
	filePath := filepath.Join(fbs.rootPath, filepath.Clean(key))
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("blob not found: %w", err)
	}
	meta := BlobMetadata{
		Size:      int64(len(data)),
		Checksum:  fmt.Sprintf("%x", md5.Sum(data)),
		UpdatedAt: time.Now(),
		CreatedAt: time.Now(),
	}
	metaPath := filePath + ".meta"
	if b, err := os.ReadFile(metaPath); err == nil {
		_ = json.Unmarshal(b, &meta)
	}
	return &BlobData{Key: key, Data: data, Metadata: meta}, nil
}

func (fbs *FilesystemBlobStorage) Delete(key string) error {
	filePath := filepath.Join(fbs.rootPath, filepath.Clean(key))
	_ = os.Remove(filePath + ".meta")
	return os.Remove(filePath)
}

func (fbs *FilesystemBlobStorage) Exists(key string) (bool, error) {
	filePath := filepath.Join(fbs.rootPath, filepath.Clean(key))
	_, err := os.Stat(filePath)
	return err == nil, nil
}

func (fbs *FilesystemBlobStorage) List(prefix string) ([]BlobInfo, error) {
	var infos []BlobInfo
	walk := func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || strings.HasSuffix(path, ".meta") {
			return nil
		}
		rel, _ := filepath.Rel(fbs.rootPath, path)
		if prefix == "" || strings.HasPrefix(rel, prefix) {
			fi, _ := os.Stat(path)
			infos = append(infos, BlobInfo{Key: rel, Metadata: BlobMetadata{Size: fi.Size(), CreatedAt: fi.ModTime(), UpdatedAt: fi.ModTime()}})
		}
		return nil
	}
	if err := filepath.WalkDir(fbs.rootPath, walk); err != nil {
		return nil, err
	}
	return infos, nil
}

func (fbs *FilesystemBlobStorage) Stats() (BlobStats, error) {
	var totalBlobs, totalSize int64
	walk := func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasSuffix(path, ".meta") {
			return nil
		}
		fi, _ := os.Stat(path)
		totalBlobs++
		totalSize += fi.Size()
		return nil
	}
	if err := filepath.WalkDir(fbs.rootPath, walk); err != nil {
		return BlobStats{}, err
	}
	return BlobStats{TotalBlobs: totalBlobs, TotalSize: totalSize, UsedSpace: totalSize}, nil
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: blobonly -root <dir> <command> [options]\n")
	fmt.Fprintf(os.Stderr, "Commands: put|get|del|list|stat\n")
}

func main() {
	root := flag.String("root", "", "Root directory for blobs")
	flag.Parse()
	if *root == "" || flag.NArg() < 1 {
		usage()
		os.Exit(2)
	}
	store, err := NewFilesystemBlobStorage(*root, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	cmd := flag.Arg(0)
	switch cmd {
	case "put":
		if flag.NArg() < 2 {
			fmt.Fprintln(os.Stderr, "put <key> [-f file] [-ct type] [-fn filename]")
			os.Exit(2)
		}
		key := flag.Arg(1)
		file := ""
		ct := "application/octet-stream"
		fn := ""
		args := flag.Args()[2:]
		for i := 0; i < len(args); i++ {
			switch args[i] {
			case "-f":
				i++
				if i < len(args) {
					file = args[i]
				}
			case "-ct":
				i++
				if i < len(args) {
					ct = args[i]
				}
			case "-fn":
				i++
				if i < len(args) {
					fn = args[i]
				}
			}
		}
		var data []byte
		if file != "" {
			b, err := os.ReadFile(file)
			if err != nil {
				fmt.Fprintln(os.Stderr, "read file:", err)
				os.Exit(1)
			}
			data = b
		} else {
			b, err := io.ReadAll(os.Stdin)
			if err != nil {
				fmt.Fprintln(os.Stderr, "stdin:", err)
				os.Exit(1)
			}
			data = b
		}
		meta := BlobMetadata{ContentType: ct, Filename: fn}
		if err := store.Store(key, data, meta); err != nil {
			fmt.Fprintln(os.Stderr, "store:", err)
			os.Exit(1)
		}
		fmt.Println("OK")
	case "get":
		if flag.NArg() < 2 {
			fmt.Fprintln(os.Stderr, "get <key> [-o output]")
			os.Exit(2)
		}
		key := flag.Arg(1)
		out := ""
		args := flag.Args()[2:]
		for i := 0; i < len(args); i++ {
			if args[i] == "-o" {
				i++
				if i < len(args) {
					out = args[i]
				}
			}
		}
		blob, err := store.Retrieve(key)
		if err != nil {
			fmt.Fprintln(os.Stderr, "retrieve:", err)
			os.Exit(1)
		}
		if out == "" {
			os.Stdout.Write(blob.Data)
		} else {
			if err := os.WriteFile(out, blob.Data, 0o644); err != nil {
				fmt.Fprintln(os.Stderr, "write:", err)
				os.Exit(1)
			}
		}
	case "del":
		if flag.NArg() < 2 {
			fmt.Fprintln(os.Stderr, "del <key>")
			os.Exit(2)
		}
		key := flag.Arg(1)
		if err := store.Delete(key); err != nil {
			fmt.Fprintln(os.Stderr, "delete:", err)
			os.Exit(1)
		}
		fmt.Println("OK")
	case "list":
		prefix := ""
		if flag.NArg() >= 2 {
			prefix = flag.Arg(1)
		}
		infos, err := store.List(prefix)
		if err != nil {
			fmt.Fprintln(os.Stderr, "list:", err)
			os.Exit(1)
		}
		for _, info := range infos {
			fmt.Printf("%s\t%d bytes\n", info.Key, info.Metadata.Size)
		}
	case "stat":
		stats, err := store.Stats()
		if err != nil {
			fmt.Fprintln(os.Stderr, "stats:", err)
			os.Exit(1)
		}
		fmt.Printf("blobs=%d size=%d bytes\n", stats.TotalBlobs, stats.TotalSize)
	default:
		usage()
		os.Exit(2)
	}
}
