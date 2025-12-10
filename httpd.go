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
const TIMEOUT = 5
const MAX_REQUESTS = 100

func main() {

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", PORT))
	if err != nil {
		// handle error
		log.Fatal(err)
	}
	log.Printf("Server is up ! Go to http://localhost:%d", PORT)

	for {
		conn, err := ln.Accept()
		if err != nil {
			// handle error
			log.Fatal(err)
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	// Close the connection when the function finishes
	defer conn.Close()

	// Use bufio to read the connection's data line by line without too much effort
	reader := bufio.NewReader(conn)

	// Set initial timeout
	conn.SetReadDeadline(time.Now().Add(TIMEOUT * time.Second))

	// Loop requests over the connection for Keep-Alive
	for requestCounter := 0; requestCounter < MAX_REQUESTS; requestCounter++ {
		status := 200

		// Read the Request-Line (e.g., "GET /hello.txt HTTP/1.1")
		requestLine, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
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
		}

		// Access log
		fmt.Printf("[%s] %s - %s %d - %s\n", time.Now().Format(time.RFC3339), conn.RemoteAddr(), method, status, decodedPath)

		responseHeaders := http.Header{}
		responseHeaders.Add("Server", "flo's mini httpd")
		responseHeaders.Add("Date", time.Now().UTC().Format(time.RFC1123))
		responseHeaders.Add("Content-Type", contentType)

		// Set keep-alive header
		if headers.Get("Connection") == "keep-alive" {
			responseHeaders.Add("Connection", "keep-alive")
			responseHeaders.Add("Keep-Alive", fmt.Sprintf("timeout=%d, max=%d", TIMEOUT, MAX_REQUESTS))
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
		conn.SetReadDeadline(time.Now().Add(TIMEOUT * time.Second))
	}
}

func readHeaders(reader *bufio.Reader) http.Header {
	headers := http.Header{}

	fmt.Printf("%s", headers)

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

		log.Printf(line)

		// Empty line means we reached the end of the headers
		if line == "" {
			break
		}

		// Get KV pair
		// Split in exactly two parts because there might be colons in the values (cookies, user agent, etc)
		headerSlice := strings.SplitN(line, ": ", 2)

		if len(headerSlice) != 2 {
			log.Printf("Incorrect header slice (%d parts)", len(headerSlice))
			return nil
		}

		key := strings.TrimSpace(headerSlice[0])
		value := strings.TrimSpace(headerSlice[1])

		headers.Add(key, value)
	}

	return headers
}

func buildResponse(status int, body []byte, responseHeaders http.Header) []byte {
	header := fmt.Sprintf("HTTP/1.1 %d %s\r\n"+
		"Content-Length: %d\r\n",
		status,
		http.StatusText(status),
		len(body),
	)

	if responseHeaders != nil {
		for key, value := range responseHeaders {
			header += fmt.Sprintf("%s: %s\r\n", key, value[0])
		}
	}

	header += "\r\n"

	return append([]byte(header), body...)
}
