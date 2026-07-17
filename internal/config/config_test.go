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
				"API_TOKEN":   strings.Repeat("a", 32),
			},
			expected: Config{
				DBHost:        "postgres",
				DBPort:        5432,
				DBName:        "feedway",
				DBUser:        "feedway",
				DBPassword:    "secret",
				APIToken:      strings.Repeat("a", 32),
				RetentionDays: 60,
			},
		},
		{
			name: "custom retention",
			environment: map[string]string{
				"DB_HOST":        "postgres",
				"DB_NAME":        "feedway",
				"DB_USER":        "feedway",
				"DB_PASSWORD":    "secret",
				"API_TOKEN":      strings.Repeat("a", 32),
				"RETENTION_DAYS": "90",
			},
			expected: Config{
				DBHost:        "postgres",
				DBPort:        5432,
				DBName:        "feedway",
				DBUser:        "feedway",
				DBPassword:    "secret",
				APIToken:      strings.Repeat("a", 32),
				RetentionDays: 90,
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
				"API_TOKEN":   strings.Repeat("a", 32),
			},
			expected: Config{
				DBHost:        "db.example",
				DBPort:        5433,
				DBName:        "feed/way",
				DBUser:        "feed@way",
				DBPassword:    "p@ss/word",
				APIToken:      strings.Repeat("a", 32),
				RetentionDays: 60,
			},
		},
		{
			name: "database port invalid",
			environment: map[string]string{
				"DB_HOST":     "postgres",
				"DB_PORT":     "65536",
				"DB_NAME":     "feedway",
				"DB_USER":     "feedway",
				"DB_PASSWORD": "secret",
				"API_TOKEN":   strings.Repeat("a", 32),
			},
			expectedError: "DB_PORT must be between 1 and 65535",
		},
		{
			name: "API token too short",
			environment: map[string]string{
				"DB_HOST":     "postgres",
				"DB_NAME":     "feedway",
				"DB_USER":     "feedway",
				"DB_PASSWORD": "secret",
				"API_TOKEN":   strings.Repeat("a", 31),
			},
			expectedError: "API_TOKEN must be at least 32 bytes",
		},
		{
			name: "retention is not an integer",
			environment: map[string]string{
				"DB_HOST":        "postgres",
				"DB_NAME":        "feedway",
				"DB_USER":        "feedway",
				"DB_PASSWORD":    "secret",
				"API_TOKEN":      strings.Repeat("a", 32),
				"RETENTION_DAYS": "many",
			},
			expectedError: "RETENTION_DAYS must be a positive integer",
		},
		{
			name: "retention is zero",
			environment: map[string]string{
				"DB_HOST":        "postgres",
				"DB_NAME":        "feedway",
				"DB_USER":        "feedway",
				"DB_PASSWORD":    "secret",
				"API_TOKEN":      strings.Repeat("a", 32),
				"RETENTION_DAYS": "0",
			},
			expectedError: "RETENTION_DAYS must be a positive integer",
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
