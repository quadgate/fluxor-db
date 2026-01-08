package main

import (
	"context"
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// BlobStorage provides interface for storing binary large objects
type BlobStorage interface {
	Store(ctx context.Context, key string, data []byte, metadata BlobMetadata) error
	Retrieve(ctx context.Context, key string) (*BlobData, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	List(ctx context.Context, prefix string) ([]BlobInfo, error)
	Stats(ctx context.Context) (BlobStats, error)
}

// BlobMetadata contains metadata about stored blobs
type BlobMetadata struct {
	ContentType string            `json:"content_type"`
	Filename    string            `json:"filename,omitempty"`
	Size        int64             `json:"size"`
	Checksum    string            `json:"checksum"`
	Tags        map[string]string `json:"tags,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// BlobData represents retrieved blob with metadata
type BlobData struct {
	Key      string       `json:"key"`
	Data     []byte       `json:"-"`
	Metadata BlobMetadata `json:"metadata"`
}

// BlobInfo contains summary information about a blob
type BlobInfo struct {
	Key      string       `json:"key"`
	Metadata BlobMetadata `json:"metadata"`
}

// BlobStats contains storage statistics
type BlobStats struct {
	TotalBlobs int64 `json:"total_blobs"`
	TotalSize  int64 `json:"total_size"`
	UsedSpace  int64 `json:"used_space"`
}

// BlobStorageConfig configures blob storage backend
type BlobStorageConfig struct {
	Backend     string // "database", "filesystem", "memory"
	RootPath    string // For filesystem backend
	TableName   string // For database backend
	MaxSize     int64  // Maximum blob size
	Compression bool   // Enable compression
}

// DatabaseBlobStorage stores blobs in database BLOB fields
type DatabaseBlobStorage struct {
	runtime   *DBRuntime
	tableName string
	maxSize   int64
}

// NewDatabaseBlobStorage creates database-backed blob storage
func NewDatabaseBlobStorage(runtime *DBRuntime, config *BlobStorageConfig) (*DatabaseBlobStorage, error) {
	tableName := "blobs"
	if config.TableName != "" {
		tableName = config.TableName
	}

	maxSize := int64(100 * 1024 * 1024) // 100MB default
	if config.MaxSize > 0 {
		maxSize = config.MaxSize
	}

	storage := &DatabaseBlobStorage{
		runtime:   runtime,
		tableName: tableName,
		maxSize:   maxSize,
	}

	// Create table if not exists
	if err := storage.createTable(); err != nil {
		return nil, fmt.Errorf("failed to create blob table: %w", err)
	}

	return storage, nil
}

// createTable creates the blob storage table
func (dbs *DatabaseBlobStorage) createTable() error {
	ctx := context.Background()

	// Create table based on database type
	var createSQL string
	switch dbs.runtime.config.DatabaseType {
	case DatabaseTypeSQLite:
		createSQL = fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				key TEXT PRIMARY KEY,
				data BLOB NOT NULL,
				content_type TEXT NOT NULL,
				filename TEXT,
				size INTEGER NOT NULL,
				checksum TEXT NOT NULL,
				tags TEXT, -- JSON
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)`, dbs.tableName)
	case DatabaseTypePostgreSQL:
		createSQL = fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				key TEXT PRIMARY KEY,
				data BYTEA NOT NULL,
				content_type TEXT NOT NULL,
				filename TEXT,
				size BIGINT NOT NULL,
				checksum TEXT NOT NULL,
				tags JSONB,
				created_at TIMESTAMP DEFAULT NOW(),
				updated_at TIMESTAMP DEFAULT NOW()
			)`, dbs.tableName)
	case DatabaseTypeMySQL:
		createSQL = fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				` + "`key`" + ` VARCHAR(255) PRIMARY KEY,
				data LONGBLOB NOT NULL,
				content_type VARCHAR(255) NOT NULL,
				filename VARCHAR(255),
				size BIGINT NOT NULL,
				checksum VARCHAR(64) NOT NULL,
				tags JSON,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
			)`, dbs.tableName)
	default:
		return fmt.Errorf("unsupported database type for blob storage: %s", dbs.runtime.config.DatabaseType)
	}

	_, err := dbs.runtime.Exec(ctx, createSQL)
	return err
}

// Store stores a blob in the database
func (dbs *DatabaseBlobStorage) Store(ctx context.Context, key string, data []byte, metadata BlobMetadata) error {
	if len(data) > int(dbs.maxSize) {
		return fmt.Errorf("blob size %d exceeds maximum %d", len(data), dbs.maxSize)
	}

	// Calculate checksum
	checksum := fmt.Sprintf("%x", md5.Sum(data))
	if metadata.Checksum != "" && metadata.Checksum != checksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", metadata.Checksum, checksum)
	}
	metadata.Checksum = checksum
	metadata.Size = int64(len(data))
	metadata.UpdatedAt = time.Now()
	if metadata.CreatedAt.IsZero() {
		metadata.CreatedAt = metadata.UpdatedAt
	}

	// Serialize tags if present
	var tagsJSON string
	if len(metadata.Tags) > 0 {
		// Simple JSON serialization for tags
		parts := make([]string, 0, len(metadata.Tags))
		for k, v := range metadata.Tags {
			parts = append(parts, fmt.Sprintf(`"%s":"%s"`, k, v))
		}
		tagsJSON = "{" + strings.Join(parts, ",") + "}"
	}

	// Insert or update
	if dbs.runtime.config.DatabaseType == DatabaseTypeMySQL {
		_, err := dbs.runtime.Exec(ctx, fmt.Sprintf(`
			REPLACE INTO %s (` + "`key`" + `, data, content_type, filename, size, checksum, tags, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, dbs.tableName),
			key, data, metadata.ContentType, metadata.Filename, metadata.Size,
			metadata.Checksum, tagsJSON, metadata.CreatedAt, metadata.UpdatedAt)
		return err
	} else {
		_, err := dbs.runtime.Exec(ctx, fmt.Sprintf(`
			INSERT OR REPLACE INTO %s (key, data, content_type, filename, size, checksum, tags, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, dbs.tableName),
			key, data, metadata.ContentType, metadata.Filename, metadata.Size,
			metadata.Checksum, tagsJSON, metadata.CreatedAt, metadata.UpdatedAt)
		return err
	}
}

// Retrieve retrieves a blob from the database
func (dbs *DatabaseBlobStorage) Retrieve(ctx context.Context, key string) (*BlobData, error) {
	row := dbs.runtime.QueryRow(ctx, fmt.Sprintf(`
		SELECT data, content_type, filename, size, checksum, tags, created_at, updated_at
		FROM %s WHERE key = ?
	`, dbs.tableName), key)

	var data []byte
	var contentType, filename, checksum, tagsJSON string
	var size int64
	var createdAt, updatedAt time.Time

	err := row.Scan(&data, &contentType, &filename, &size, &checksum, &tagsJSON, &createdAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("blob not found: %w", err)
	}

	// Parse tags
	tags := make(map[string]string)
	if tagsJSON != "" {
		// Simple JSON parsing for tags (basic implementation)
		tagsJSON = strings.Trim(tagsJSON, "{}")
		if tagsJSON != "" {
			pairs := strings.Split(tagsJSON, ",")
			for _, pair := range pairs {
				parts := strings.SplitN(pair, ":", 2)
				if len(parts) == 2 {
					key := strings.Trim(parts[0], `"`)
					value := strings.Trim(parts[1], `"`)
					tags[key] = value
				}
			}
		}
	}

	return &BlobData{
		Key:  key,
		Data: data,
		Metadata: BlobMetadata{
			ContentType: contentType,
			Filename:    filename,
			Size:        size,
			Checksum:    checksum,
			Tags:        tags,
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
		},
	}, nil
}

// Delete removes a blob from storage
func (dbs *DatabaseBlobStorage) Delete(ctx context.Context, key string) error {
	_, err := dbs.runtime.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE key = ?", dbs.tableName), key)
	return err
}

// Exists checks if a blob exists
func (dbs *DatabaseBlobStorage) Exists(ctx context.Context, key string) (bool, error) {
	row := dbs.runtime.QueryRow(ctx, fmt.Sprintf("SELECT 1 FROM %s WHERE key = ?", dbs.tableName), key)
	var exists int
	err := row.Scan(&exists)
	if err != nil {
		return false, nil // Not found
	}
	return true, nil
}

// List lists blobs with optional prefix filter
func (dbs *DatabaseBlobStorage) List(ctx context.Context, prefix string) ([]BlobInfo, error) {
	var query string
	var args []interface{}

	if prefix != "" {
		if dbs.runtime.config.DatabaseType == DatabaseTypePostgreSQL {
			query = fmt.Sprintf("SELECT key, content_type, filename, size, checksum, tags, created_at, updated_at FROM %s WHERE key LIKE $1", dbs.tableName)
		} else {
			query = fmt.Sprintf("SELECT key, content_type, filename, size, checksum, tags, created_at, updated_at FROM %s WHERE key LIKE ?", dbs.tableName)
		}
		args = []interface{}{prefix + "%"}
	} else {
		query = fmt.Sprintf("SELECT key, content_type, filename, size, checksum, tags, created_at, updated_at FROM %s", dbs.tableName)
	}

	rows, err := dbs.runtime.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var infos []BlobInfo
	for rows.Next() {
		var key, contentType, filename, checksum, tagsJSON string
		var size int64
		var createdAt, updatedAt time.Time

		err := rows.Scan(&key, &contentType, &filename, &size, &checksum, &tagsJSON, &createdAt, &updatedAt)
		if err != nil {
			continue
		}

		// Parse tags
		tags := make(map[string]string)
		if tagsJSON != "" {
			tagsJSON = strings.Trim(tagsJSON, "{}")
			if tagsJSON != "" {
				pairs := strings.Split(tagsJSON, ",")
				for _, pair := range pairs {
					parts := strings.SplitN(pair, ":", 2)
					if len(parts) == 2 {
						tagKey := strings.Trim(parts[0], `"`)
						tagValue := strings.Trim(parts[1], `"`)
						tags[tagKey] = tagValue
					}
				}
			}
		}

		infos = append(infos, BlobInfo{
			Key: key,
			Metadata: BlobMetadata{
				ContentType: contentType,
				Filename:    filename,
				Size:        size,
				Checksum:    checksum,
				Tags:        tags,
				CreatedAt:   createdAt,
				UpdatedAt:   updatedAt,
			},
		})
	}

	return infos, nil
}

// Stats returns storage statistics
func (dbs *DatabaseBlobStorage) Stats(ctx context.Context) (BlobStats, error) {
	row := dbs.runtime.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*), COALESCE(SUM(size), 0) FROM %s", dbs.tableName))

	var totalBlobs, totalSize int64
	err := row.Scan(&totalBlobs, &totalSize)
	if err != nil {
		return BlobStats{}, err
	}

	return BlobStats{
		TotalBlobs: totalBlobs,
		TotalSize:  totalSize,
		UsedSpace:  totalSize, // Same as total size for database storage
	}, nil
}

// FilesystemBlobStorage stores blobs on filesystem
type FilesystemBlobStorage struct {
	rootPath string
	maxSize  int64
}

// NewFilesystemBlobStorage creates filesystem-backed blob storage
func NewFilesystemBlobStorage(config *BlobStorageConfig) (*FilesystemBlobStorage, error) {
	if config.RootPath == "" {
		return nil, fmt.Errorf("root path is required for filesystem blob storage")
	}

	// Create root directory if it doesn't exist
	if err := os.MkdirAll(config.RootPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create root directory: %w", err)
	}

	maxSize := int64(100 * 1024 * 1024) // 100MB default
	if config.MaxSize > 0 {
		maxSize = config.MaxSize
	}

	return &FilesystemBlobStorage{
		rootPath: config.RootPath,
		maxSize:  maxSize,
	}, nil
}

// Store stores a blob on filesystem
func (fbs *FilesystemBlobStorage) Store(ctx context.Context, key string, data []byte, metadata BlobMetadata) error {
	if len(data) > int(fbs.maxSize) {
		return fmt.Errorf("blob size %d exceeds maximum %d", len(data), fbs.maxSize)
	}

	// Create subdirectories based on key
	filePath := filepath.Join(fbs.rootPath, key)
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write blob data
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write blob: %w", err)
	}

	// Write metadata
	metadataPath := filePath + ".meta"
	metadataJSON := fmt.Sprintf(`{
		"content_type": "%s",
		"filename": "%s",
		"size": %d,
		"checksum": "%x",
		"created_at": "%s",
		"updated_at": "%s"
	}`,
		metadata.ContentType,
		metadata.Filename,
		len(data),
		md5.Sum(data),
		time.Now().Format(time.RFC3339),
		time.Now().Format(time.RFC3339))

	return os.WriteFile(metadataPath, []byte(metadataJSON), 0644)
}

// Retrieve retrieves a blob from filesystem
func (fbs *FilesystemBlobStorage) Retrieve(ctx context.Context, key string) (*BlobData, error) {
	filePath := filepath.Join(fbs.rootPath, key)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("blob not found: %w", err)
	}

	// Read metadata if exists
	metadata := BlobMetadata{
		Size:      int64(len(data)),
		Checksum:  fmt.Sprintf("%x", md5.Sum(data)),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Try to read metadata file
	metadataPath := filePath + ".meta"
	if metaData, err := os.ReadFile(metadataPath); err == nil {
		// Parse basic metadata (simplified)
		metaStr := string(metaData)
		if strings.Contains(metaStr, "image/") {
			metadata.ContentType = "image/jpeg" // Default
			if strings.Contains(metaStr, "png") {
				metadata.ContentType = "image/png"
			} else if strings.Contains(metaStr, "gif") {
				metadata.ContentType = "image/gif"
			}
		}
	}

	return &BlobData{
		Key:      key,
		Data:     data,
		Metadata: metadata,
	}, nil
}

// Delete removes a blob from filesystem
func (fbs *FilesystemBlobStorage) Delete(ctx context.Context, key string) error {
	filePath := filepath.Join(fbs.rootPath, key)
	os.Remove(filePath + ".meta") // Remove metadata if exists
	return os.Remove(filePath)
}

// Exists checks if blob exists on filesystem
func (fbs *FilesystemBlobStorage) Exists(ctx context.Context, key string) (bool, error) {
	filePath := filepath.Join(fbs.rootPath, key)
	_, err := os.Stat(filePath)
	return err == nil, nil
}

// List lists blobs on filesystem
func (fbs *FilesystemBlobStorage) List(ctx context.Context, prefix string) ([]BlobInfo, error) {
	var infos []BlobInfo

	err := filepath.Walk(fbs.rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || strings.HasSuffix(path, ".meta") {
			return nil
		}

		relPath, _ := filepath.Rel(fbs.rootPath, path)
		if prefix == "" || strings.HasPrefix(relPath, prefix) {
			infos = append(infos, BlobInfo{
				Key: relPath,
				Metadata: BlobMetadata{
					Size:      info.Size(),
					CreatedAt: info.ModTime(),
					UpdatedAt: info.ModTime(),
				},
			})
		}
		return nil
	})

	return infos, err
}

// Stats returns filesystem storage statistics
func (fbs *FilesystemBlobStorage) Stats(ctx context.Context) (BlobStats, error) {
	var totalBlobs int64
	var totalSize int64

	err := filepath.Walk(fbs.rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || strings.HasSuffix(path, ".meta") {
			return nil
		}
		totalBlobs++
		totalSize += info.Size()
		return nil
	})

	return BlobStats{
		TotalBlobs: totalBlobs,
		TotalSize:  totalSize,
		UsedSpace:  totalSize,
	}, err
}