/*
	This is a smol and cute HTTP/1.1 static web server !
	It only supports GET and static files and currently basically no headers

    HTTP/1.1 protocol specification: https://www.rfc-editor.org/rfc/rfc7230
*/

package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
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

		// 1) Parse the incoming request
		request, err := parseRequest(reader)
		if err != nil {
			if os.IsTimeout(err) {
				// Timeouts are expected when connections are idle - don't log them
				return
			}
			log.Printf("Error parsing request: %v", err)
			conn.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n<h1>400 Bad Request</h1>\r\n\r\n"))
			return
		}

		// 2) Get information about the targeted resource
		resourceInfo, err := getResourceInfo(request.Path, config)
		if os.IsNotExist(err) {
			log.Printf("404 found by getResourceInfo: %v", err)
			conn.Write([]byte("HTTP/1.1 404 Not Found\r\n\r\n<h1>404 Not Found</h1>\r\n\r\n"))
			return
		}
		// if os.IsPermission(err) {
		// 	conn.Write([]byte("HTTP/1.1 403 Forbidden\r\n\r\n<h1>403 Forbidden</h1>\r\n\r\n"))
		// 	return
		// }
		if err != nil {
			log.Printf("Error getting resource info: %v", err)
			conn.Write([]byte("HTTP/1.1 500 Internal Server Error\r\n\r\n<h1>500 Internal Server Error</h1>\r\n\r\n"))
			return
		}

		fmt.Printf("Resource Info: %+v\n", resourceInfo)

		// 3) Build the response
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

		// Close the connection if the Connection header is set to "close" or missing
		if request.Headers.Get("Connection") == "close" || request.Headers.Get("Connection") == "" {
			break
		}

		// Set additional timeout to wait for the next request
		conn.SetReadDeadline(time.Now().Add(time.Duration(config.timeout) * time.Second))
	}
}
