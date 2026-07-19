package config

import (
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
)

const (
	defaultDatabasePort  = 5432
	defaultHTTPPort      = 80
	defaultRetentionDays = 60
	apiTokenLength       = 64
)

type Config struct {
	HTTPPort      uint16
	BaseURL       string
	DBHost        string
	DBPort        uint16
	DBName        string
	DBUser        string
	DBPassword    string
	APIToken      string
	RetentionDays int
}

type LookupEnv func(string) (string, bool)

func Load(lookupEnv LookupEnv) (Config, error) {
	database, err := loadDatabase(lookupEnv)
	if err != nil {
		return Config{}, err
	}
	httpPort, err := loadPort(lookupEnv, "HTTP_PORT", defaultHTTPPort)
	if err != nil {
		return Config{}, err
	}
	apiToken, _ := lookupEnv("API_TOKEN")
	baseURL, _ := lookupEnv("BASE_URL")

	if len(apiToken) != apiTokenLength {
		return Config{}, errors.New("API_TOKEN must be 64 hexadecimal characters")
	}
	if _, err := hex.DecodeString(apiToken); err != nil {
		return Config{}, errors.New("API_TOKEN must be 64 hexadecimal characters")
	}
	retentionDays, err := loadRetentionDays(lookupEnv)
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPPort:      httpPort,
		BaseURL:       strings.TrimRight(baseURL, "/"),
		DBHost:        database.host,
		DBPort:        database.port,
		DBName:        database.name,
		DBUser:        database.user,
		DBPassword:    database.password,
		APIToken:      apiToken,
		RetentionDays: retentionDays,
	}, nil
}

type databaseConfig struct {
	host     string
	port     uint16
	name     string
	user     string
	password string
}

func loadDatabase(lookupEnv LookupEnv) (databaseConfig, error) {
	host, _ := lookupEnv("DB_HOST")
	name, _ := lookupEnv("DB_NAME")
	user, _ := lookupEnv("DB_USER")
	password, _ := lookupEnv("DB_PASSWORD")

	required := []struct {
		name  string
		value string
	}{
		{name: "DB_HOST", value: host},
		{name: "DB_NAME", value: name},
		{name: "DB_USER", value: user},
		{name: "DB_PASSWORD", value: password},
	}
	for _, variable := range required {
		if strings.TrimSpace(variable.value) == "" {
			return databaseConfig{}, errors.New(variable.name + " is required")
		}
	}

	port, err := loadPort(lookupEnv, "DB_PORT", defaultDatabasePort)
	if err != nil {
		return databaseConfig{}, err
	}

	return databaseConfig{
		host:     host,
		port:     port,
		name:     name,
		user:     user,
		password: password,
	}, nil
}

func loadPort(lookupEnv LookupEnv, name string, defaultPort uint16) (uint16, error) {
	value, _ := lookupEnv(name)
	if value == "" {
		return defaultPort, nil
	}

	port, err := strconv.ParseUint(value, 10, 16)
	if err != nil || port == 0 {
		return 0, errors.New(name + " must be between 1 and 65535")
	}

	return uint16(port), nil
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
