package config

import (
	"errors"
	"strconv"
	"strings"
)

const (
	defaultDatabasePort  = 5432
	defaultRetentionDays = 60
)

type Config struct {
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
	apiToken, _ := lookupEnv("API_TOKEN")

	if len(apiToken) < 32 {
		return Config{}, errors.New("API_TOKEN must be at least 32 bytes")
	}
	retentionDays, err := loadRetentionDays(lookupEnv)
	if err != nil {
		return Config{}, err
	}

	return Config{
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

	port := uint16(defaultDatabasePort)
	if value, _ := lookupEnv("DB_PORT"); value != "" {
		parsedPort, err := strconv.ParseUint(value, 10, 16)
		if err != nil || parsedPort == 0 {
			return databaseConfig{}, errors.New("DB_PORT must be between 1 and 65535")
		}
		port = uint16(parsedPort)
	}

	return databaseConfig{
		host:     host,
		port:     port,
		name:     name,
		user:     user,
		password: password,
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
