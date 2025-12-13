package main

import (
	"fmt"
	"net/http"
)

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
