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

func getResourceInfo(path string) (ResourceInfo, error) {
	fileInfo, err := os.Stat(path)

	if err != nil {
		return ResourceInfo{}, err
	}

	return ResourceInfo{
		LocalFilePath: path,
		ContentType: mime.TypeByExtension(filepath.Ext(path)),
		ETag: generateETag(path, fileInfo.ModTime()),
		LastModified: fileInfo.ModTime(),
		IsDirectory: fileInfo.IsDir(),
		IsFile: !fileInfo.IsDir(),
		FileSize: fileInfo.Size(),
		IsExecutable: fileInfo.Mode()&0111 != 0,
		IsReadable: fileInfo.Mode()&0400 != 0,
		IsWritable: fileInfo.Mode()&0200 != 0,
	}, nil
}

func generateETag(path string, lastModified time.Time) string {
	// Generate an ETag based on the path and the last modified time
	return fmt.Sprintf("\"%x\"", getFNVHash([]byte(fmt.Sprintf("%d-%s", lastModified.Unix(), path))))
}

func getFNVHash(blob []byte) uint64 {
	// Fast non cryptographic hash
	// https://hackernoon.com/modern-hashing-with-go-a-guide
	h := fnv.New64a()
	h.Write(blob)
	return h.Sum64()
}
