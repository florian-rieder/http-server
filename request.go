package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
)

type Request struct {
	Method string
	Path string
	Headers http.Header
}

func parseRequest(reader *bufio.Reader) (Request, error) {
	method, rawEncodedPath, httpVersion, err := parseRequestLine(reader)
	if err != nil {
		return Request{}, err
	}

	if method == "" || rawEncodedPath == "" || httpVersion == "" {
		return Request{}, fmt.Errorf("invalid request line")
	}

	if method != "GET" {
		return Request{}, fmt.Errorf("unsupported method: '%s'", method)
	}
	if httpVersion != "HTTP/1.1" {
		return Request{}, fmt.Errorf("unsupported HTTP Version: '%s'", httpVersion)
	}

	headers := parseRequestHeaders(reader)
	if headers == nil {
		return Request{}, fmt.Errorf("invalid request headers")
	}

	// Decode path
	decodedPath, err := url.QueryUnescape(rawEncodedPath)
	if err != nil {
		return Request{}, fmt.Errorf("invalid path: %w", err)
	}

	if decodedPath == "/" {
		// default to index.html
		decodedPath = "/index.html"
	}

	// Clean the path to prevent path traversal attacks and other URL shenanigans
	cleanPath := filepath.Clean(decodedPath)

	return Request{
		Method: method,
		Path: cleanPath,
		Headers: headers,
	}, nil
}

func parseRequestLine(reader *bufio.Reader) (string, string, string, error) {
	// Read the Request-Line (e.g., "GET /hello.txt HTTP/1.1")
	requestLine, err := reader.ReadString('\n')
	if err != nil {
		if err != io.EOF {
			return "", "", "", err
		}
		return "", "", "", fmt.Errorf("unexpected EOF in request-line")
	}

	// Strip newline
	requestLine = strings.TrimSpace(requestLine)

	if requestLine == "" {
		return "", "", "", fmt.Errorf("empty request line")
	}

	// Split the string into a slice of strings
	parts := strings.Split(requestLine, " ")

	// Check the length of the resulting slice
	if len(parts) < 3 {
		log.Printf("Error: Invalid request line format.\nRequest line: '%s'", requestLine)
		return "", "", "", fmt.Errorf("invalid request line format")
	}

	method := parts[0] // For now only GET is supported
	rawEncodedPath := parts[1]
	httpVersion := parts[2]

	return method, rawEncodedPath, httpVersion, nil
}

func parseRequestHeaders(reader *bufio.Reader) http.Header {
	headers := http.Header{}

	// Process the headers
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			// Check for error conditions
			if err != io.EOF {
				log.Printf("Error reading from connection: %v", err)
			}
			break
		}

		// Remove newline
		line = strings.TrimSpace(line)

		log.Println(line)

		// Empty line means we reached the end of the headers (since we removed CRLF with the trim)
		if line == "" {
			break
		}

		// Get KV pair
		// Split in exactly two parts because there might be colons in the values (cookies, user agent, etc)
		headerSlice := strings.SplitN(line, ":", 2)

		if len(headerSlice) != 2 {
			log.Printf("Incorrect header slice (%d parts)\nHeader: '%s'", len(headerSlice), line)
			return nil
		}

		key := strings.TrimSpace(headerSlice[0])
		value := strings.TrimSpace(headerSlice[1])

		headers.Add(key, value)
	}

	return headers
}