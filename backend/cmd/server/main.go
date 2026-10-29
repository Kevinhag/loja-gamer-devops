// Comando server: sobe a API da K.O Gaming Store e serve o frontend compilado.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lojagamer/internal/api"
	"lojagamer/internal/config"
	"lojagamer/internal/database"
	"lojagamer/internal/images"
	"lojagamer/internal/store"
)

// version é preenchida no build: -ldflags "-X main.version=<commit>".
var version = "dev"

func main() {
	healthcheck := flag.Bool("healthcheck", false, "consulta /livez e sai com código 0 se o servidor estiver de pé (HEALTHCHECK do Docker)")
	flag.Parse()

	if *healthcheck {
		os.Exit(runHealthcheck())
	}
	if err := run(); err != nil {
		slog.Error("servidor encerrado com erro", "erro", err.Error())
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuração inválida: %w", err)
	}
	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := database.Open(ctx, cfg.DSN(false), 90*time.Second)
	if err != nil {
		return err
	}
	defer db.Close()
	logger.Info("conectado ao MySQL", "host", cfg.DBHost, "database", cfg.DBName)

	if cfg.Migrate {
		migDB, err := database.Open(ctx, cfg.DSN(true), 10*time.Second)
		if err != nil {
			return err
		}
		err = database.Migrate(ctx, migDB)
		migDB.Close()
		if err != nil {
			return err
		}
	}

	storage, err := newStorage(ctx, cfg)
	if err != nil {
		return err
	}
	if cfg.SeedImagesDir != "" {
		seedCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		n, err := images.Seed(seedCtx, storage, cfg.SeedImagesDir)
		cancel()
		if err != nil {
			// Não impede a subida: o /healthz vai acusar o problema de storage.
			logger.Error("falha ao enviar imagens iniciais", "erro", err.Error())
		} else {
			logger.Info("imagens iniciais verificadas", "enviadas", n, "driver", cfg.StorageDriver)
		}
	}

	if cfg.AdminToken == "" {
		logger.Warn("ADMIN_TOKEN não definido: a área admin ficará desabilitada")
	}

	srv := api.New(api.Options{
		Store:        store.NewMySQL(db),
		Images:       storage,
		Logger:       logger,
		AdminToken:   cfg.AdminToken,
		CookieSecure: cfg.CookieSecure,
		StaticDir:    cfg.StaticDir,
		Version:      version,
	})
	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("servidor iniciado", "port", cfg.Port, "storage", cfg.StorageDriver)
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		logger.Info("sinal recebido, encerrando")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}

func newStorage(ctx context.Context, cfg config.Config) (images.Storage, error) {
	if cfg.StorageDriver == "s3" {
		s, err := images.NewS3(ctx, cfg.AWSRegion, cfg.S3Bucket, cfg.S3Prefix)
		if err != nil {
			return nil, fmt.Errorf("configurando S3: %w", err)
		}
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := s.Ping(pingCtx); err != nil {
			slog.Error("bucket S3 inacessível", "bucket", cfg.S3Bucket, "erro", err.Error())
		}
		return s, nil
	}
	return images.NewLocal(cfg.ImagesDir)
}

func newLogger(level string) *slog.Logger {
	var l slog.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		l = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l})).
		With("service", "lojagamer", "version", version)
}

func runHealthcheck() int {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/livez")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
