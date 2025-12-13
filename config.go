package main

import (
	"log"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	port                        int
	document_root               string
	timeout                     int
	max_requests_per_connection int
}

func loadConfig(config_path string) Config {
	c := Config{}

	fileContent, err := os.ReadFile(config_path)
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}

	// Parse the config file
	lines := strings.Split(string(fileContent), "\n")
	for _, line := range lines {
		parts := strings.Split(line, "=")
		if len(parts) != 2 {
			log.Fatalf("Invalid config line: %s", line)
		}
		key := parts[0]
		value := parts[1]
		switch key {
		case "Port":
			c.port, err = strconv.Atoi(value)
			if err != nil {
				log.Fatalf("Invalid port: %s", value)
			}
		case "DocumentRoot":
			c.document_root = value
		case "Timeout":
			c.timeout, err = strconv.Atoi(value)
			if err != nil {
				log.Fatalf("Invalid timeout: %s", value)
			}
		case "MaxRequestsPerConnection":
			c.max_requests_per_connection, err = strconv.Atoi(value)
			if err != nil {
				log.Fatalf("Invalid max requests per connection: %s", value)
			}
		default:
			log.Fatalf("Invalid config key: %s", key)
		}
	}

	return c
}
