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
				"DATABASE_URL": "postgres://feedway:secret@postgres/feedway",
				"API_TOKEN":    strings.Repeat("a", 32),
			},
			expected: Config{
				DatabaseURL:   "postgres://feedway:secret@postgres/feedway",
				APIToken:      strings.Repeat("a", 32),
				RetentionDays: 60,
			},
		},
		{
			name: "custom retention",
			environment: map[string]string{
				"DATABASE_URL":   "postgres://feedway:secret@postgres/feedway",
				"API_TOKEN":      strings.Repeat("a", 32),
				"RETENTION_DAYS": "90",
			},
			expected: Config{
				DatabaseURL:   "postgres://feedway:secret@postgres/feedway",
				APIToken:      strings.Repeat("a", 32),
				RetentionDays: 90,
			},
		},
		{
			name:          "database URL missing",
			environment:   map[string]string{"API_TOKEN": strings.Repeat("a", 32)},
			expectedError: "DATABASE_URL is required",
		},
		{
			name: "database URL malformed",
			environment: map[string]string{
				"DATABASE_URL": "://bad",
				"API_TOKEN":    strings.Repeat("a", 32),
			},
			expectedError: "DATABASE_URL is invalid",
		},
		{
			name: "database URL scheme unsupported",
			environment: map[string]string{
				"DATABASE_URL": "https://postgres/feedway",
				"API_TOKEN":    strings.Repeat("a", 32),
			},
			expectedError: "DATABASE_URL must use postgres or postgresql scheme",
		},
		{
			name: "database URL host missing",
			environment: map[string]string{
				"DATABASE_URL": "postgres:///feedway",
				"API_TOKEN":    strings.Repeat("a", 32),
			},
			expectedError: "DATABASE_URL must include a host",
		},
		{
			name: "API token too short",
			environment: map[string]string{
				"DATABASE_URL": "postgres://postgres/feedway",
				"API_TOKEN":    strings.Repeat("a", 31),
			},
			expectedError: "API_TOKEN must be at least 32 bytes",
		},
		{
			name: "retention is not an integer",
			environment: map[string]string{
				"DATABASE_URL":   "postgres://postgres/feedway",
				"API_TOKEN":      strings.Repeat("a", 32),
				"RETENTION_DAYS": "many",
			},
			expectedError: "RETENTION_DAYS must be a positive integer",
		},
		{
			name: "retention is zero",
			environment: map[string]string{
				"DATABASE_URL":   "postgres://postgres/feedway",
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
