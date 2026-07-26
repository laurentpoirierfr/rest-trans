package main

import (
	"context"
	"database/sql"
	"log"
	"strings"
	"time"

	"github.com/laurentpoirierfr/rest-trans/internal/api"
	"github.com/laurentpoirierfr/rest-trans/internal/config"
	"github.com/laurentpoirierfr/rest-trans/internal/schema"
	"github.com/laurentpoirierfr/rest-trans/internal/transaction"

	_ "github.com/lib/pq"
)

func main() {
	cfg := config.Load()

	log.Printf("Connecting to PostgreSQL at %s:%d/%s...", cfg.Database.Host, cfg.Database.Port, cfg.Database.Name)

	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := retryPingDB(func() error { return db.Ping() }, 15, 2*time.Second); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Database connected successfully")

	db.SetMaxOpenConns(cfg.Database.Pool.MaxOpen)
	db.SetMaxIdleConns(cfg.Database.Pool.MaxIdle)
	if cfg.Database.Pool.ConnMaxLife > 0 {
		db.SetConnMaxLifetime(cfg.Database.Pool.ConnMaxLife)
	}
	if cfg.Database.Pool.ConnMaxIdle > 0 {
		db.SetConnMaxIdleTime(cfg.Database.Pool.ConnMaxIdle)
	}

	log.Printf("Pool: max_open=%d, max_idle=%d, conn_max_life=%v, conn_max_idle=%v",
		cfg.Database.Pool.MaxOpen, cfg.Database.Pool.MaxIdle,
		cfg.Database.Pool.ConnMaxLife, cfg.Database.Pool.ConnMaxIdle)

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
		log.Printf("Introspecting schema '%s'...", s)

		schemaData, err := schema.Introspect(db, s)
		if err != nil {
			log.Fatalf("Failed to introspect schema '%s': %v", s, err)
		}

		for _, table := range schemaData {
			store.AddTable(s, table)
		}
		totalTables += len(schemaData)
		log.Printf("Schema '%s': found %d tables/views", s, len(schemaData))

		funcs, err := schema.IntrospectFunctions(db, s)
		if err != nil {
			log.Printf("Warning: failed to introspect functions for schema '%s': %v", s, err)
		} else {
			for _, fn := range funcs {
				store.AddFunction(s, fn)
			}
			totalFunctions += len(funcs)
			log.Printf("Schema '%s': found %d functions", s, len(funcs))
		}
	}

	log.Printf("Total exposed: %d tables/views, %d functions", totalTables, totalFunctions)
	for _, sName := range store.SchemaNames() {
		for name, table := range store.TablesBySchema(sName) {
			if table.AllowedMethods != nil {
				log.Printf("  - /%s/%s %v", sName, name, table.AllowedMethods)
			} else {
				log.Printf("  - /%s/%s", sName, name)
			}
		}
	}
	for name := range store.AllFunctions() {
		log.Printf("  - /rpc/%s", name)
	}

	var txManager *transaction.Manager
	if cfg.Transactions.Enabled {
		txManager = transaction.NewManager(db, cfg.Transactions.TTL)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		txManager.StartCleanup(ctx, cfg.Transactions.CleanupInterval)
		log.Printf("Transactions enabled: TTL=%v, cleanup_interval=%v", cfg.Transactions.TTL, cfg.Transactions.CleanupInterval)
	}

	router := api.NewRouter(db, store, schemas, cfg, txManager)

	addr := cfg.ServerAddr()
	log.Printf("Server starting on %s", addr)

	if err := router.Run(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
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
