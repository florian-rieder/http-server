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
	"os"
	"time"
)

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
			serveErrorDocument(conn, config, 400) // Bad Request
			return
		}

		// 2) Get information about the targeted resource
		resourceInfo, err := getResourceInfo(request.Path, config)
		if os.IsNotExist(err) {
			serveErrorDocument(conn, config, 404) // Not Found
			return
		}
		if os.IsPermission(err) {
		 	serveErrorDocument(conn, config, 404) // Forbidden, but maybe don't tell them ?
			return
		}
		if err != nil {
			log.Printf("Error getting resource info: %v", err)
			serveErrorDocument(conn, config, 500) // Internal Server Error
			return
		}

		// 3) Build the response
		serveBufferedResponse(conn, config, request, resourceInfo)

		// Close the connection if the Connection header is set to "close" or missing
		if request.Headers.Get("Connection") == "close" || request.Headers.Get("Connection") == "" {
			break
		}

		// Set additional timeout to wait for the next request
		conn.SetReadDeadline(time.Now().Add(time.Duration(config.timeout) * time.Second))
	}
}
