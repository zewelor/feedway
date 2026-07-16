//go:build integration

package feedway

import (
	"context"
	"net"
	"net/url"
	"os"
	"testing"
	"time"
)

func TestPostgresIsReachable(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("DATABASE_URL is required")
	}

	parsedURL, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatalf("parse DATABASE_URL: %v", err)
	}

	host := parsedURL.Hostname()
	port := parsedURL.Port()
	if port == "" {
		port = "5432"
	}

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(host, port))
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	if err := connection.Close(); err != nil {
		t.Fatalf("close PostgreSQL connection: %v", err)
	}
}
