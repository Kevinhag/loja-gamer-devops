package config

import (
	"strings"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	for _, k := range []string{"PORT", "DB_HOST", "STORAGE_DRIVER", "MIGRATE", "DB_TLS"} {
		t.Setenv(k, "")
	}
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Port != "8080" || c.DBHost != "localhost" || c.StorageDriver != "local" || !c.Migrate {
		t.Fatalf("defaults inesperados: %+v", c)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("PORT", "9000")
	t.Setenv("DB_HOST", "meu-rds.amazonaws.com")
	t.Setenv("STORAGE_DRIVER", "S3")
	t.Setenv("S3_BUCKET", "loja-imagens")
	t.Setenv("MIGRATE", "false")
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Port != "9000" || c.DBHost != "meu-rds.amazonaws.com" || c.StorageDriver != "s3" || c.Migrate {
		t.Fatalf("config não refletiu o ambiente: %+v", c)
	}
}

func TestValidate(t *testing.T) {
	base := Config{Port: "8080", StorageDriver: "local", ImagesDir: "x", DBTLS: "false"}
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{"válida", func(*Config) {}, ""},
		{"porta inválida", func(c *Config) { c.Port = "abc" }, "PORT"},
		{"s3 sem bucket", func(c *Config) { c.StorageDriver = "s3" }, "S3_BUCKET"},
		{"driver desconhecido", func(c *Config) { c.StorageDriver = "ftp" }, "STORAGE_DRIVER"},
		{"tls inválido", func(c *Config) { c.DBTLS = "talvez" }, "DB_TLS"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := base
			tt.mutate(&c)
			err := c.Validate()
			if tt.wantErr == "" && err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("esperava erro contendo %q, veio %v", tt.wantErr, err)
			}
		})
	}
}

func TestDSN(t *testing.T) {
	c := Config{DBUser: "admin", DBPassword: "p@ss:word", DBHost: "db", DBPort: "3306", DBName: "lojagamer", DBTLS: "false"}
	dsn := c.DSN(false)
	if !strings.HasPrefix(dsn, "admin:p@ss:word@tcp(db:3306)/lojagamer?") {
		t.Fatalf("DSN inesperado: %s", dsn)
	}
	if !strings.Contains(dsn, "parseTime=true") || strings.Contains(dsn, "multiStatements") {
		t.Fatalf("parâmetros inesperados: %s", dsn)
	}
	if !strings.Contains(c.DSN(true), "multiStatements=true") {
		t.Fatal("DSN de migration deveria ter multiStatements")
	}
}
