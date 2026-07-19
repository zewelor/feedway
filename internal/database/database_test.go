package database

import "testing"

func TestPoolConfig(t *testing.T) {
	t.Setenv("PGHOST", "ambient.example")
	t.Setenv("PGPORT", "9999")
	t.Setenv("PGDATABASE", "ambient")
	t.Setenv("PGUSER", "ambient")
	t.Setenv("PGPASSWORD", "ambient")

	database := Config{
		Host:     "db.example",
		Port:     5433,
		Name:     "feed/way",
		User:     "feed@way",
		Password: "p@ss/word",
		SSLMode:  "verify-full",
	}

	config, err := poolConfig(database)
	if err != nil {
		t.Fatalf("poolConfig() error = %v", err)
	}
	if config.ConnConfig.Host != database.Host {
		t.Errorf("host = %q, want %q", config.ConnConfig.Host, database.Host)
	}
	if config.ConnConfig.Port != database.Port {
		t.Errorf("port = %d, want %d", config.ConnConfig.Port, database.Port)
	}
	if config.ConnConfig.Database != database.Name {
		t.Errorf("database = %q, want %q", config.ConnConfig.Database, database.Name)
	}
	if config.ConnConfig.User != database.User {
		t.Errorf("user = %q, want %q", config.ConnConfig.User, database.User)
	}
	if config.ConnConfig.Password != database.Password {
		t.Error("password was not preserved")
	}
	if config.ConnConfig.TLSConfig == nil {
		t.Fatal("TLS config = nil, want verify-full TLS")
	}
	if config.ConnConfig.TLSConfig.ServerName != database.Host {
		t.Errorf("TLS server name = %q, want %q", config.ConnConfig.TLSConfig.ServerName, database.Host)
	}
	if config.MaxConns != 4 {
		t.Errorf("max connections = %d, want 4", config.MaxConns)
	}
}

func TestPoolConfigDisablesTLSExplicitly(t *testing.T) {
	config, err := poolConfig(Config{
		Host:     "postgres",
		Port:     5432,
		Name:     "feedway",
		User:     "feedway",
		Password: "secret",
		SSLMode:  "disable",
	})
	if err != nil {
		t.Fatalf("poolConfig() error = %v", err)
	}
	if config.ConnConfig.TLSConfig != nil {
		t.Fatal("TLS config is set, want explicit plaintext connection")
	}
	if len(config.ConnConfig.Fallbacks) != 0 {
		t.Fatalf("fallbacks = %d, want none", len(config.ConnConfig.Fallbacks))
	}
}
