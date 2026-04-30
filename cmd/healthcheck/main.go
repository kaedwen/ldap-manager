package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	// Health check URL - use port 9090 by default
	healthURL := os.Getenv("HEALTH_URL")
	if healthURL == "" {
		healthURL = "http://localhost:9090/health"
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	// Make health check request
	resp, err := client.Get(healthURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Health check failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Health check returned status %d\n", resp.StatusCode)
		os.Exit(1)
	}

	// Success
	os.Exit(0)
}
