package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

func serveBufferedResponse(conn net.Conn, config Config, request Request, resourceInfo ResourceInfo) {
	encoding := "identity"
	status := 200

	fileContent, err := os.ReadFile(resourceInfo.LocalFilePath)
	if err != nil {
		conn.Write([]byte("HTTP/1.1 500 Internal Server Error\r\n\r\n<h1>500 Internal Server Error</h1>\r\n\r\n"))
		return
	}
	// The client sends the header "If-None-Match" with the last Etag they have in memory for this URL
	// The server compares the client's ETag (sent with If-None-Match) with the ETag for its
	// current version of the resource, and if both values match (that is, the resource has
	// not changed), the server sends back a 304 Not Modified status, without a body, which
	// tells the client that the cached version of the response is still good to use (fresh).
	// see https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/ETag
	clientEtag := request.Headers.Get("If-None-Match")
	if clientEtag != "" {
		if clientEtag == resourceInfo.ETag {
			// Yup, we got the same file, just use that !
			status = 304
			fileContent = []byte("") // Ensure no body is sent
		}
	}

	// Check if the client accepts gzip compression
	acceptedEncodings := strings.Split(request.Headers.Get("Accept-Encoding"), ",")
	for _, acceptedEncoding := range acceptedEncodings {
		acceptedEncoding = strings.TrimSpace(acceptedEncoding)
		if acceptedEncoding == "gzip" {
			compressedContent, err := gzipBytes(fileContent)
			if err != nil {
				log.Printf("Error compressing response: %v", err)
			}
			fileContent = compressedContent
			encoding = "gzip"
			break
		}
	}

	// Access log
	fmt.Printf("[%s] %d %s %s - %s (%s)\n", time.Now().Format(time.RFC3339), status, request.Method, request.Path, conn.RemoteAddr(), request.Headers.Get("User-Agent"))

	now := time.Now().UTC().Format(time.RFC1123)
	responseHeaders := http.Header{}
	responseHeaders.Add("Server", "flo's mini httpd")
	responseHeaders.Add("Date", now)
	responseHeaders.Add("Expires", now)
	responseHeaders.Add("Content-Type", resourceInfo.ContentType)

	if resourceInfo.ETag != "" {
		responseHeaders.Add("ETag", resourceInfo.ETag)
	}

	// If the file was compressed, add the appropriate headers to tell the client
	// how to decompress it.
	if encoding == "gzip" {
		responseHeaders.Add("Content-Encoding", "gzip")
		responseHeaders.Add("Vary", "Accept-Encoding")
	}

	// Set keep-alive header to notify the client that the connection is still open and can be reused
	if request.Headers.Get("Connection") == "keep-alive" {
		responseHeaders.Add("Connection", "keep-alive")
		responseHeaders.Add("Keep-Alive", fmt.Sprintf("timeout=%d, max=%d", config.timeout, config.max_requests_per_connection))
	} else {
		responseHeaders.Add("Connection", "close")
	}

	response := buildResponse(status, fileContent, responseHeaders)

	conn.Write([]byte(response))
}

func buildResponse(status int, body []byte, responseHeaders http.Header) []byte {
	header := fmt.Sprintf("HTTP/1.1 %d %s\r\n"+
		"Content-Length: %d\r\n",
		status,
		http.StatusText(status),
		len(body),
	)

	for key, value := range responseHeaders {
		header += fmt.Sprintf("%s: %s\r\n", key, value[0])
	}

	// Signify the end of the header section using CRLF
	header += "\r\n"

	// Combine header and body
	response := append([]byte(header), body...)
	return response
}

func serveErrorDocument(conn net.Conn, config Config, status int) {
	statusText := http.StatusText(status)

	// Default body; simple, efficient, refined.
	body := fmt.Sprintf("<h1>%d %s</h1>\r\n", status, statusText)

	// If an error document is configured, use it instead of the default body
	switch status {
	case 403:
		if config.error_document_403 != "" {
			body = readErrorDocument(config.error_document_403)
		}
	case 404:
		if config.error_document_404 != "" {
			body = readErrorDocument(config.error_document_404)
		}
	case 500:
		if config.error_document_500 != "" {
			body = readErrorDocument(config.error_document_500)
		}
	}

	responseHeaders := http.Header{}
	responseHeaders.Add("Server", "flo's mini httpd")
	responseHeaders.Add("Date", time.Now().UTC().Format(time.RFC1123))
	responseHeaders.Add("Content-Type", "text/html")
	responseHeaders.Add("Connection", "close")

	response := buildResponse(status, []byte(body), responseHeaders)
	conn.Write([]byte(response))
}

func readErrorDocument(path string) string {
	fileContent, err := os.ReadFile(path)
	if err != nil {
		log.Printf("Error reading error document: %v", err)
		return ""
	}
	return string(fileContent)
}