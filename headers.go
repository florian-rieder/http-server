package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

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

		fmt.Println(line)

		// Empty line means we reached the end of the headers (since we removed CRLF with the trim)
		if line == "" {
			break
		}

		// Get KV pair
		// Split in exactly two parts because there might be colons in the values (cookies, user agent, etc)
		headerSlice := strings.SplitN(line, ":", 2)

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