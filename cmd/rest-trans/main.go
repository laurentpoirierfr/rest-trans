package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/laurentpoirierfr/rest-trans/internal/api"
	"github.com/laurentpoirierfr/rest-trans/internal/config"
	"github.com/laurentpoirierfr/rest-trans/internal/schema"
	"github.com/laurentpoirierfr/rest-trans/internal/transaction"

	_ "github.com/lib/pq"
)

func main() {
	cfg := config.Load()

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	slog.Info("connecting to database",
		"host", cfg.Database.Host,
		"port", cfg.Database.Port,
		"database", cfg.Database.Name,
	)

	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := retryPingDB(func() error { return db.Ping() }, 15, 2*time.Second); err != nil {
		slog.Error("failed to ping database", "error", err)
		os.Exit(1)
	}
	slog.Info("database connected")

	db.SetMaxOpenConns(cfg.Database.Pool.MaxOpen)
	db.SetMaxIdleConns(cfg.Database.Pool.MaxIdle)
	if cfg.Database.Pool.ConnMaxLife > 0 {
		db.SetConnMaxLifetime(cfg.Database.Pool.ConnMaxLife)
	}
	if cfg.Database.Pool.ConnMaxIdle > 0 {
		db.SetConnMaxIdleTime(cfg.Database.Pool.ConnMaxIdle)
	}

	slog.Info("connection pool configured",
		"max_open", cfg.Database.Pool.MaxOpen,
		"max_idle", cfg.Database.Pool.MaxIdle,
		"conn_max_life", cfg.Database.Pool.ConnMaxLife,
		"conn_max_idle", cfg.Database.Pool.ConnMaxIdle,
	)

	schemas := cfg.Schemas()
	defaultSchema := "public"
	if len(schemas) > 0 {
		defaultSchema = strings.TrimSpace(schemas[0])
	}

	store := schema.NewSchemaStore(defaultSchema)
	totalTables := 0
	totalFunctions := 0

	for _, s := range schemas {
		s = strings.TrimSpace(s)
		slog.Info("introspecting schema", "schema", s)

		schemaData, err := schema.Introspect(db, s)
		if err != nil {
			slog.Error("failed to introspect schema", "schema", s, "error", err)
			os.Exit(1)
		}

		for _, table := range schemaData {
			store.AddTable(s, table)
		}
		totalTables += len(schemaData)
		slog.Info("schema introspected", "schema", s, "tables", len(schemaData))

		funcs, err := schema.IntrospectFunctions(db, s)
		if err != nil {
			slog.Warn("failed to introspect functions", "schema", s, "error", err)
		} else {
			for _, fn := range funcs {
				store.AddFunction(s, fn)
			}
			totalFunctions += len(funcs)
			slog.Info("functions introspected", "schema", s, "functions", len(funcs))
		}
	}

	slog.Info("introspection complete", "tables", totalTables, "functions", totalFunctions)
	for _, sName := range store.SchemaNames() {
		for name, table := range store.TablesBySchema(sName) {
			if table.AllowedMethods != nil {
				slog.Info("endpoint", "path", "/"+sName+"/"+name, "methods", table.AllowedMethods)
			} else {
				slog.Info("endpoint", "path", "/"+sName+"/"+name)
			}
		}
	}
	for name := range store.AllFunctions() {
		slog.Info("rpc endpoint", "path", "/rpc/"+name)
	}

	var txManager *transaction.Manager
	if cfg.Transactions.Enabled {
		txManager = transaction.NewManager(db, cfg.Transactions.TTL)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		txManager.StartCleanup(ctx, cfg.Transactions.CleanupInterval)
		slog.Info("transactions enabled",
			"ttl", cfg.Transactions.TTL,
			"cleanup_interval", cfg.Transactions.CleanupInterval,
		)
	}

	router := api.NewRouter(db, store, schemas, cfg, txManager)

	addr := cfg.ServerAddr()
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("server starting", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-quit
	slog.Info("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}

	if txManager != nil {
		txManager.StopCleanup()
	}

	db.Close()
	slog.Info("server stopped")
}

func retryPingDB(ping func() error, attempts int, delay time.Duration) error {
	var lastErr error

	for i := 0; i < attempts; i++ {
		if err := ping(); err == nil {
			return nil
		} else {
			lastErr = err
		}

		if i < attempts-1 && delay > 0 {
			time.Sleep(delay)
		}
	}

	return lastErr
}
