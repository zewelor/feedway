package config

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const defaultRetentionDays = 60

type Config struct {
	DatabaseURL   string
	APIToken      string
	RetentionDays int
}

type LookupEnv func(string) (string, bool)

func Load(lookupEnv LookupEnv) (Config, error) {
	databaseURL, _ := lookupEnv("DATABASE_URL")
	apiToken, _ := lookupEnv("API_TOKEN")

	if err := validateDatabaseURL(databaseURL); err != nil {
		return Config{}, err
	}
	if len(apiToken) < 32 {
		return Config{}, errors.New("API_TOKEN must be at least 32 bytes")
	}
	retentionDays, err := loadRetentionDays(lookupEnv)
	if err != nil {
		return Config{}, err
	}

	return Config{
		DatabaseURL:   databaseURL,
		APIToken:      apiToken,
		RetentionDays: retentionDays,
	}, nil
}

func loadRetentionDays(lookupEnv LookupEnv) (int, error) {
	value, _ := lookupEnv("RETENTION_DAYS")
	if value == "" {
		return defaultRetentionDays, nil
	}

	days, err := strconv.Atoi(value)
	if err != nil || days < 1 {
		return 0, errors.New("RETENTION_DAYS must be a positive integer")
	}

	return days, nil
}

func validateDatabaseURL(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("DATABASE_URL is required")
	}

	parsedURL, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("DATABASE_URL is invalid: %w", err)
	}
	if parsedURL.Scheme != "postgres" && parsedURL.Scheme != "postgresql" {
		return errors.New("DATABASE_URL must use postgres or postgresql scheme")
	}
	if parsedURL.Host == "" {
		return errors.New("DATABASE_URL must include a host")
	}

	return nil
}
