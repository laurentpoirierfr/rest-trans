package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/laurentpoirierfr/rest-trans/internal/api"
	"github.com/laurentpoirierfr/rest-trans/internal/config"
	"github.com/laurentpoirierfr/rest-trans/internal/schema"
	"github.com/laurentpoirierfr/rest-trans/internal/transaction"
)

type TestSuite struct {
	DB        *sql.DB
	ServerURL string
	cancel    context.CancelFunc
}

func SetupSuite() *TestSuite {
	gin.SetMode(gin.TestMode)

	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		log.Fatalf("Failed to start postgres container: %v", err)
	}

	host, _ := pgContainer.Host(ctx)
	port, _ := pgContainer.MappedPort(ctx, "5432")
	portInt, _ := strconv.Atoi(port.Port())
	dsn := fmt.Sprintf("host=%s port=%s user=testuser password=testpass dbname=testdb sslmode=disable", host, port.Port())

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		pgContainer.Terminate(ctx)
		log.Fatalf("Failed to open database: %v", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		pgContainer.Terminate(ctx)
		log.Fatalf("Failed to ping database: %v", err)
	}

	schemaDir := filepath.Join("..", "infras", "init-db")
	for _, file := range []string{"schema.sql", "store-proc.sql", "data.sql"} {
		content, err := os.ReadFile(filepath.Join(schemaDir, file))
		if err != nil {
			db.Close()
			pgContainer.Terminate(ctx)
			log.Fatalf("Failed to read %s: %v", file, err)
		}
		if _, err := db.Exec(string(content)); err != nil {
			db.Close()
			pgContainer.Terminate(ctx)
			log.Fatalf("Failed to execute %s: %v", file, err)
		}
	}

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 0,
		},
		Database: config.DatabaseConfig{
			Host:     host,
			Port:     portInt,
			User:     "testuser",
			Password: "testpass",
			Name:     "testdb",
			Schemas:  []string{"public"},
			SSLMode:  "disable",
			Pool: config.PoolConfig{
				MaxOpen:     10,
				MaxIdle:     5,
				ConnMaxLife: 1 * time.Hour,
				ConnMaxIdle: 10 * time.Minute,
			},
		},
		Permissions: map[string]config.PermConfig{
			"*": {Methods: []string{"*"}},
		},
		RPC: map[string]config.RPCConfig{
			"*": {Enabled: true},
		},
		Transactions: config.TransactionsConfig{
			Enabled:         true,
			TTL:             30 * time.Minute,
			CleanupInterval: 1 * time.Minute,
		},
	}

	schemas := cfg.Schemas()
	defaultSchema := "public"
	if len(schemas) > 0 {
		defaultSchema = strings.TrimSpace(schemas[0])
	}

	store := schema.NewSchemaStore(defaultSchema)
	for _, s := range schemas {
		s = strings.TrimSpace(s)
		tables, err := schema.Introspect(db, s)
		if err != nil {
			db.Close()
			pgContainer.Terminate(ctx)
			log.Fatalf("Failed to introspect schema '%s': %v", s, err)
		}
		for _, table := range tables {
			table.AllowedMethods = nil
			table.DenyMethods = nil
			store.AddTable(s, table)
		}

		funcs, err := schema.IntrospectFunctions(db, s)
		if err != nil {
			fmt.Printf("Warning: failed to introspect functions for schema '%s': %v\n", s, err)
		} else {
			for _, fn := range funcs {
				store.AddFunction(s, fn)
			}
		}
	}

	txManager := transaction.NewManager(db, cfg.Transactions.TTL)
	txCtx, txCancel := context.WithCancel(context.Background())
	txManager.StartCleanup(txCtx, cfg.Transactions.CleanupInterval)

	router := api.NewRouter(db, store, schemas, cfg, txManager, nil)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		txCancel()
		db.Close()
		pgContainer.Terminate(ctx)
		log.Fatalf("Failed to create listener: %v", err)
	}

	server := &http.Server{Handler: router}
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	suite := &TestSuite{
		DB:        db,
		ServerURL: fmt.Sprintf("http://%s", listener.Addr().String()),
	}

	teardown := func() {
		server.Close()
		txCancel()
		db.Close()
		pgContainer.Terminate(context.Background())
	}

	_ = teardown
	registerTeardown(teardown)

	return suite
}

var teardownFuncs []func()

func registerTeardown(fn func()) {
	teardownFuncs = append(teardownFuncs, fn)
}

func TeardownAll() {
	for i := len(teardownFuncs) - 1; i >= 0; i-- {
		teardownFuncs[i]()
	}
}
