/*
	This is a smol and cute HTTP/1.1 static web server !
	It only supports GET and static files and currently basically no headers

    HTTP/1.1 protocol specification: https://www.rfc-editor.org/rfc/rfc7230
*/

package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const DOCUMENT_ROOT = "/Users/frieder/Documents/go/httpd/html"
const PORT = 8080
const TIMEOUT = 10
const MAX_REQUESTS = 100

func main() {

	config := loadConfig("./main.conf")

	fmt.Printf("Port: %d; Root: %s; Timeout: %d; Max Requests: %d\n", config.port, config.document_root, config.timeout, config.max_requests_per_connection)
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", config.port))
	if err != nil {
		// handle error
		log.Fatal(err)
	}
	log.Printf("Server is up ! Go to http://localhost:%d", config.port)

	for {
		conn, err := ln.Accept()
		if err != nil {
			// handle errors
			log.Println(err)
			continue
		}
		go handleConnection(conn, config)
	}
}

func handleConnection(conn net.Conn, config Config) {
	// Close the connection when the function finishes
	defer conn.Close()

	// Use bufio to read the connection's data line by line without too much effort
	reader := bufio.NewReader(conn)

	// Set initial timeout
	conn.SetReadDeadline(time.Now().Add(time.Duration(config.timeout) * time.Second))

	// Loop requests over the connection for Keep-Alive
	for requestCounter := 0; requestCounter < config.max_requests_per_connection; requestCounter++ {
		status := 200

		// Read the Request-Line (e.g., "GET /hello.txt HTTP/1.1")
		requestLine, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				// Timeouts are expected when connections are idle - don't log them
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					return
				}
				log.Printf("Error reading from connection: %v", err)
			} else {
				log.Printf("Unexpected EOF in request-line")
			}
		}

		// Strip newline
		requestLine = strings.TrimSpace(requestLine)

		if requestLine == "" {
			break
		}

		// Split the string into a slice of strings
		parts := strings.Split(requestLine, " ")

		// Check the length of the resulting slice
		if len(parts) < 3 {
			fmt.Println("Error: Invalid request line format.")
			return
		}

		method := parts[0] // For now only GET is supported
		rawEncodedPath := parts[1]
		httpVersion := parts[2]

		if method != "GET" {
			conn.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
			log.Printf("Unsupported method: '%s'", method)
			return
		}
		if httpVersion != "HTTP/1.1" {
			conn.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
			log.Printf("Unsupported HTTP Version: '%s'", httpVersion)
			return
		}

		headers := readHeaders(reader)

		// Decode path
		decodedPath, err := url.QueryUnescape(rawEncodedPath)
		if err != nil {
			conn.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
			return
		}

		if decodedPath == "/" {
			// default to index.html
			decodedPath = "/index.html"
		}

		contentType := "text/plain"
		cleanPath := filepath.Clean(decodedPath)
		localFilePath := filepath.Join(DOCUMENT_ROOT, cleanPath)
		etag := ""
		gzipCompressed := false

		fileContent, err := os.ReadFile(localFilePath)
		if err != nil {
			if os.IsNotExist(err) {
				status = 404
				fileContent = []byte("<h1>404 Not Found</h1>")
				contentType = "text/html"
			} else {
				// e.g., permission error
				status = 500
				fileContent = []byte("<h1>500 Internal Server Error</h1>")
				contentType = "text/html"
				log.Printf("File I/O Error: %v", err)
			}
		} else {
			// Determine MIME type from file extension
			ext := filepath.Ext(localFilePath)
			contentType = mime.TypeByExtension(ext)
			status = 200
			// Generate Entity Tag for caching
			etag = fmt.Sprintf("\"%x\"", getFNVHash(fileContent))

			// The client sends the header "If-None-Match" with the last Etag they have in memory for this URL
			// The server compares the client's ETag (sent with If-None-Match) with the ETag for its
			// current version of the resource, and if both values match (that is, the resource has
			// not changed), the server sends back a 304 Not Modified status, without a body, which
			// tells the client that the cached version of the response is still good to use (fresh).
			// see https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/ETag
			clientEtag := headers.Get("If-None-Match")
			if clientEtag != "" {
				if clientEtag == etag {
					// Yup, we got the same file, just use that !
					status = 304
					fileContent = []byte("") // Ensure no body is sent
				}
			}

			// Check if the client accepts gzip compression
			acceptedEncodings := strings.Split(headers.Get("Accept-Encoding"), ",")
			for _, encoding := range acceptedEncodings {
				encoding = strings.TrimSpace(encoding)
				if encoding == "gzip" {
					compressedContent, err := gzipBytes(fileContent)
					if err != nil {
						log.Printf("Error compressing response: %v", err)
					}
					fileContent = compressedContent
					gzipCompressed = true
					break
				}
			}
		}

		// Access log
		fmt.Printf("[%s] %s - %s %d - %s\n", time.Now().Format(time.RFC3339), conn.RemoteAddr(), method, status, decodedPath)

		now := time.Now().UTC().Format(time.RFC1123)
		responseHeaders := http.Header{}
		responseHeaders.Add("Server", "flo's mini httpd")
		responseHeaders.Add("Date", now)
		responseHeaders.Add("Expires", now)
		responseHeaders.Add("Content-Type", contentType)

		if etag != "" {
			responseHeaders.Add("ETag", etag)
		}

		// If the file was compressed, add the appropriate headers to tell the client
		// how to decompress it.
		if gzipCompressed {
			responseHeaders.Add("Content-Encoding", "gzip")
			responseHeaders.Add("Vary", "Accept-Encoding")
		}

		// Set keep-alive header
		if headers.Get("Connection") == "keep-alive" {
			responseHeaders.Add("Connection", "keep-alive")
			responseHeaders.Add("Keep-Alive", fmt.Sprintf("timeout=%d, max=%d", config.timeout, config.max_requests_per_connection))
		} else {
			responseHeaders.Add("Connection", "close")
		}

		response := buildResponse(status, fileContent, responseHeaders)

		conn.Write([]byte(response))

		// Close the connection if the Connection header is set to "close" or missing
		if headers.Get("Connection") == "close" || headers.Get("Connection") == "" {
			break
		}

		// Set additional timeout to wait for the next request
		conn.SetReadDeadline(time.Now().Add(time.Duration(config.timeout) * time.Second))
	}
}
