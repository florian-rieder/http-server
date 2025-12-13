package main

import (
	"bytes"
	"compress/gzip"
	"log"
)

func gzipBytes(data []byte) ([]byte, error) {
	var buf bytes.Buffer

	gzipWriter := gzip.NewWriter(&buf)
	_, err := gzipWriter.Write(data)
	if err != nil {
		log.Printf("Error compressing response: %v", err)
		return nil, err
	}
	gzipWriter.Close()
	return buf.Bytes(), nil
}