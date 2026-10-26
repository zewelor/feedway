package config

import (
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		environment   map[string]string
		expected      Config
		expectedError string
	}{
		{
			name: "valid configuration",
			environment: map[string]string{
				"DB_HOST":     "postgres",
				"DB_NAME":     "feedway",
				"DB_USER":     "feedway",
				"DB_PASSWORD": "secret",
				"API_TOKEN":   strings.Repeat("a", 64),
			},
			expected: Config{
				HTTPPort:               80,
				DBHost:                 "postgres",
				DBPort:                 5432,
				DBName:                 "feedway",
				DBUser:                 "feedway",
				DBPassword:             "secret",
				DBSSLMode:              "disable",
				APIToken:               strings.Repeat("a", 64),
				RetentionDays:          30,
				FeedResponseMaxEntries: 100,
			},
		},
		{
			name: "custom HTTP port, base URL, retention, and feed count",
			environment: map[string]string{
				"DB_HOST":                   "postgres",
				"DB_NAME":                   "feedway",
				"DB_USER":                   "feedway",
				"DB_PASSWORD":               "secret",
				"API_TOKEN":                 strings.Repeat("a", 64),
				"BASE_URL":                  "https://feed.example.com/",
				"HTTP_PORT":                 "8080",
				"RETENTION_DAYS":            "90",
				"FEED_RESPONSE_MAX_ENTRIES": "500",
			},
			expected: Config{
				HTTPPort:               8080,
				BaseURL:                "https://feed.example.com",
				DBHost:                 "postgres",
				DBPort:                 5432,
				DBName:                 "feedway",
				DBUser:                 "feedway",
				DBPassword:             "secret",
				DBSSLMode:              "disable",
				APIToken:               strings.Repeat("a", 64),
				RetentionDays:          90,
				FeedResponseMaxEntries: 500,
			},
		},
		{
			name:          "database host missing",
			environment:   map[string]string{},
			expectedError: "DB_HOST is required",
		},
		{
			name: "database name missing",
			environment: map[string]string{
				"DB_HOST": "postgres",
			},
			expectedError: "DB_NAME is required",
		},
		{
			name: "database user missing",
			environment: map[string]string{
				"DB_HOST": "postgres",
				"DB_NAME": "feedway",
			},
			expectedError: "DB_USER is required",
		},
		{
			name: "database password missing",
			environment: map[string]string{
				"DB_HOST": "postgres",
				"DB_NAME": "feedway",
				"DB_USER": "feedway",
			},
			expectedError: "DB_PASSWORD is required",
		},
		{
			name: "custom database port and credentials",
			environment: map[string]string{
				"DB_HOST":     "db.example",
				"DB_PORT":     "5433",
				"DB_NAME":     "feed/way",
				"DB_USER":     "feed@way",
				"DB_PASSWORD": "p@ss/word",
				"API_TOKEN":   strings.Repeat("a", 64),
			},
			expected: Config{
				HTTPPort:               80,
				DBHost:                 "db.example",
				DBPort:                 5433,
				DBName:                 "feed/way",
				DBUser:                 "feed@way",
				DBPassword:             "p@ss/word",
				DBSSLMode:              "disable",
				APIToken:               strings.Repeat("a", 64),
				RetentionDays:          30,
				FeedResponseMaxEntries: 100,
			},
		},
		{
			name: "custom database TLS mode",
			environment: map[string]string{
				"DB_HOST":     "db.example",
				"DB_NAME":     "feedway",
				"DB_USER":     "feedway",
				"DB_PASSWORD": "secret",
				"DB_SSLMODE":  "verify-full",
				"API_TOKEN":   strings.Repeat("a", 64),
			},
			expected: Config{
				HTTPPort:               80,
				DBHost:                 "db.example",
				DBPort:                 5432,
				DBName:                 "feedway",
				DBUser:                 "feedway",
				DBPassword:             "secret",
				DBSSLMode:              "verify-full",
				APIToken:               strings.Repeat("a", 64),
				RetentionDays:          30,
				FeedResponseMaxEntries: 100,
			},
		},
		{
			name: "database TLS mode invalid",
			environment: map[string]string{
				"DB_HOST":     "postgres",
				"DB_NAME":     "feedway",
				"DB_USER":     "feedway",
				"DB_PASSWORD": "secret",
				"DB_SSLMODE":  "prefer",
				"API_TOKEN":   strings.Repeat("a", 64),
			},
			expectedError: "DB_SSLMODE must be disable, require, verify-ca, or verify-full",
		},
		{
			name: "database port invalid",
			environment: map[string]string{
				"DB_HOST":     "postgres",
				"DB_PORT":     "65536",
				"DB_NAME":     "feedway",
				"DB_USER":     "feedway",
				"DB_PASSWORD": "secret",
				"API_TOKEN":   strings.Repeat("a", 64),
			},
			expectedError: "DB_PORT must be between 1 and 65535",
		},
		{
			name: "HTTP port invalid",
			environment: map[string]string{
				"DB_HOST":     "postgres",
				"DB_NAME":     "feedway",
				"DB_USER":     "feedway",
				"DB_PASSWORD": "secret",
				"API_TOKEN":   strings.Repeat("a", 64),
				"HTTP_PORT":   "65536",
			},
			expectedError: "HTTP_PORT must be between 1 and 65535",
		},
		{
			name: "base URL contains a path",
			environment: map[string]string{
				"DB_HOST":     "postgres",
				"DB_NAME":     "feedway",
				"DB_USER":     "feedway",
				"DB_PASSWORD": "secret",
				"API_TOKEN":   strings.Repeat("a", 64),
				"BASE_URL":    "https://feed.example.com/path",
			},
			expectedError: "BASE_URL must be an HTTP or HTTPS origin",
		},
		{
			name: "base URL contains an empty query",
			environment: map[string]string{
				"DB_HOST":     "postgres",
				"DB_NAME":     "feedway",
				"DB_USER":     "feedway",
				"DB_PASSWORD": "secret",
				"API_TOKEN":   strings.Repeat("a", 64),
				"BASE_URL":    "https://feed.example.com?",
			},
			expectedError: "BASE_URL must be an HTTP or HTTPS origin",
		},
		{
			name: "base URL uses a non-HTTP scheme",
			environment: map[string]string{
				"DB_HOST":     "postgres",
				"DB_NAME":     "feedway",
				"DB_USER":     "feedway",
				"DB_PASSWORD": "secret",
				"API_TOKEN":   strings.Repeat("a", 64),
				"BASE_URL":    "javascript:alert(1)",
			},
			expectedError: "BASE_URL must be an HTTP or HTTPS origin",
		},
		{
			name: "API token shorter than 64 characters",
			environment: map[string]string{
				"DB_HOST":     "postgres",
				"DB_NAME":     "feedway",
				"DB_USER":     "feedway",
				"DB_PASSWORD": "secret",
				"API_TOKEN":   strings.Repeat("a", 63),
			},
			expectedError: "API_TOKEN must be 64 hexadecimal characters",
		},
		{
			name: "API token longer than 64 characters",
			environment: map[string]string{
				"DB_HOST":     "postgres",
				"DB_NAME":     "feedway",
				"DB_USER":     "feedway",
				"DB_PASSWORD": "secret",
				"API_TOKEN":   strings.Repeat("a", 65),
			},
			expectedError: "API_TOKEN must be 64 hexadecimal characters",
		},
		{
			name: "API token is not hexadecimal",
			environment: map[string]string{
				"DB_HOST":     "postgres",
				"DB_NAME":     "feedway",
				"DB_USER":     "feedway",
				"DB_PASSWORD": "secret",
				"API_TOKEN":   strings.Repeat("z", 64),
			},
			expectedError: "API_TOKEN must be 64 hexadecimal characters",
		},
		{
			name: "retention is not an integer",
			environment: map[string]string{
				"DB_HOST":        "postgres",
				"DB_NAME":        "feedway",
				"DB_USER":        "feedway",
				"DB_PASSWORD":    "secret",
				"API_TOKEN":      strings.Repeat("a", 64),
				"RETENTION_DAYS": "many",
			},
			expectedError: "RETENTION_DAYS must be between 1 and 36500",
		},
		{
			name: "retention is zero",
			environment: map[string]string{
				"DB_HOST":        "postgres",
				"DB_NAME":        "feedway",
				"DB_USER":        "feedway",
				"DB_PASSWORD":    "secret",
				"API_TOKEN":      strings.Repeat("a", 64),
				"RETENTION_DAYS": "0",
			},
			expectedError: "RETENTION_DAYS must be between 1 and 36500",
		},
		{
			name: "retention exceeds maximum",
			environment: map[string]string{
				"DB_HOST":        "postgres",
				"DB_NAME":        "feedway",
				"DB_USER":        "feedway",
				"DB_PASSWORD":    "secret",
				"API_TOKEN":      strings.Repeat("a", 64),
				"RETENTION_DAYS": "36501",
			},
			expectedError: "RETENTION_DAYS must be between 1 and 36500",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			config, err := Load(func(name string) (string, bool) {
				value, exists := test.environment[name]
				return value, exists
			})
			if test.expectedError != "" {
				if err == nil || !strings.Contains(err.Error(), test.expectedError) {
					t.Fatalf("Load() error = %v, want containing %q", err, test.expectedError)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if config != test.expected {
				t.Fatalf("Load() = %#v, want %#v", config, test.expected)
			}
		})
	}
}

func TestLoadFeedResponseMaxEntries(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		value     string
		want      int
		wantError bool
	}{
		{name: "empty uses default", want: 100},
		{name: "minimum", value: "1", want: 1},
		{name: "custom", value: "500", want: 500},
		{name: "maximum", value: "1000", want: 1000},
		{name: "zero", value: "0", wantError: true},
		{name: "negative", value: "-1", wantError: true},
		{name: "not an integer", value: "many", wantError: true},
		{name: "above maximum", value: "1001", wantError: true},
		{name: "overflow", value: "99999999999999999999999", wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			environment := map[string]string{
				"DB_HOST": "postgres", "DB_NAME": "feedway", "DB_USER": "feedway",
				"DB_PASSWORD": "secret", "API_TOKEN": strings.Repeat("a", 64),
				"FEED_RESPONSE_MAX_ENTRIES": test.value,
			}
			configuration, err := Load(func(name string) (string, bool) {
				value, exists := environment[name]
				return value, exists
			})
			if test.wantError {
				if err == nil || err.Error() != "FEED_RESPONSE_MAX_ENTRIES must be between 1 and 1000" {
					t.Fatalf("Load() error = %v, want invalid FEED_RESPONSE_MAX_ENTRIES", err)
				}
				return
			}
			if err != nil || configuration.FeedResponseMaxEntries != test.want {
				t.Fatalf("FeedResponseMaxEntries = %d, error = %v; want %d",
					configuration.FeedResponseMaxEntries, err, test.want)
			}
		})
	}
}
