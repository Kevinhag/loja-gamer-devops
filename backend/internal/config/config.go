// Package config lê a configuração da aplicação a partir de variáveis de ambiente.
// Em produção (EC2) essas variáveis vêm do arquivo /opt/lojagamer/.env.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

type Config struct {
	Port string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBTLS      string // false | true | skip-verify | preferred

	Migrate bool

	StorageDriver string // local | s3
	ImagesDir     string // usado pelo driver local
	SeedImagesDir string // imagens enviadas para o storage na inicialização, se ainda não existirem
	S3Bucket      string
	S3Prefix      string
	AWSRegion     string

	StaticDir    string
	AdminToken   string
	CookieSecure bool
	LogLevel     string
}

func Load() (Config, error) {
	c := Config{
		Port:          env("PORT", "8080"),
		DBHost:        env("DB_HOST", "localhost"),
		DBPort:        env("DB_PORT", "3306"),
		DBUser:        env("DB_USER", "root"),
		DBPassword:    env("DB_PASSWORD", ""),
		DBName:        env("DB_NAME", "lojagamer"),
		DBTLS:         env("DB_TLS", "false"),
		Migrate:       envBool("MIGRATE", true),
		StorageDriver: strings.ToLower(env("STORAGE_DRIVER", "local")),
		ImagesDir:     env("IMAGES_DIR", "../seed-images"),
		SeedImagesDir: env("SEED_IMAGES_DIR", ""),
		S3Bucket:      env("S3_BUCKET", ""),
		S3Prefix:      env("S3_PREFIX", "products/"),
		AWSRegion:     env("AWS_REGION", "us-east-1"),
		StaticDir:     env("STATIC_DIR", "../frontend/dist"),
		AdminToken:    env("ADMIN_TOKEN", ""),
		CookieSecure:  envBool("COOKIE_SECURE", false),
		LogLevel:      env("LOG_LEVEL", "info"),
	}
	return c, c.Validate()
}

func (c Config) Validate() error {
	var errs []error
	if _, err := strconv.Atoi(c.Port); err != nil {
		errs = append(errs, fmt.Errorf("PORT inválida: %q", c.Port))
	}
	switch c.StorageDriver {
	case "local":
		if c.ImagesDir == "" {
			errs = append(errs, errors.New("IMAGES_DIR é obrigatório com STORAGE_DRIVER=local"))
		}
	case "s3":
		if c.S3Bucket == "" {
			errs = append(errs, errors.New("S3_BUCKET é obrigatório com STORAGE_DRIVER=s3"))
		}
	default:
		errs = append(errs, fmt.Errorf("STORAGE_DRIVER inválido: %q (use local ou s3)", c.StorageDriver))
	}
	switch c.DBTLS {
	case "false", "true", "skip-verify", "preferred":
	default:
		errs = append(errs, fmt.Errorf("DB_TLS inválido: %q", c.DBTLS))
	}
	return errors.Join(errs...)
}

// DSN monta a string de conexão do MySQL. multiStatements só é ligado para as migrations.
func (c Config) DSN(multiStatements bool) string {
	m := mysql.NewConfig()
	m.User = c.DBUser
	m.Passwd = c.DBPassword
	m.Net = "tcp"
	m.Addr = c.DBHost + ":" + c.DBPort
	m.DBName = c.DBName
	m.ParseTime = true
	m.Loc = time.UTC
	m.MultiStatements = multiStatements
	m.TLSConfig = c.DBTLS
	m.Params = map[string]string{"charset": "utf8mb4"}
	return m.FormatDSN()
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
