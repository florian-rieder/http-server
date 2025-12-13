package main

import (
	"fmt"
	"hash/fnv"
	"mime"
	"os"
	"path/filepath"
	"time"
)

type ResourceInfo struct {
	LocalFilePath string
	ContentType string
	ETag string
	LastModified time.Time
	IsDirectory bool
	IsFile bool
	FileSize int64
	IsExecutable bool
	IsReadable bool
	IsWritable bool
}

func getResourceInfo(path string, config Config) (ResourceInfo, error) {
	localFilePath := filepath.Join(config.document_root, path)
	fileInfo, err := os.Stat(localFilePath)
	if err != nil {
		return ResourceInfo{}, err
	}

	etag, err := generateETag(localFilePath, fileInfo.ModTime(), config)
	if err != nil {
		return ResourceInfo{}, err
	}

	return ResourceInfo{
		LocalFilePath: localFilePath,
		ContentType: mime.TypeByExtension(filepath.Ext(path)),
		ETag: etag,
		LastModified: fileInfo.ModTime(),
		IsDirectory: fileInfo.IsDir(),
		IsFile: !fileInfo.IsDir(),
		FileSize: fileInfo.Size(),
		IsExecutable: fileInfo.Mode()&0111 != 0,
		IsReadable: fileInfo.Mode()&0400 != 0,
		IsWritable: fileInfo.Mode()&0200 != 0,
	}, nil
}

func generateETag(path string, lastModified time.Time, config Config) (string, error) {
	if config.use_strong_etag {
		// Read file content and hash it
		fileContent, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}

		// Strong etag, worse performance (since we have to read the file) but more accurate.
		return fmt.Sprintf("\"%x\"", getFNVHash(fileContent)), nil
	} else {
		// Generate an ETag based on the path and the last modified time (weak etag)
		return fmt.Sprintf("W/\"%x\"", getFNVHash([]byte(fmt.Sprintf("%d-%s", lastModified.Unix(), path)))), nil
	}
}

func getFNVHash(blob []byte) uint64 {
	// Fast non cryptographic hash
	// https://hackernoon.com/modern-hashing-with-go-a-guide
	h := fnv.New64a()
	h.Write(blob)
	return h.Sum64()
}
