package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server       ServerConfig            `mapstructure:"server"`
	Database     DatabaseConfig          `mapstructure:"database"`
	HotReload    HotReloadConfig         `mapstructure:"hot_reload"`
	Permissions  map[string]PermConfig   `mapstructure:"permissions"`
	RPC          map[string]RPCConfig    `mapstructure:"rpc"`
	Transactions TransactionsConfig     `mapstructure:"transactions"`
	RateLimit    RateLimitConfig         `mapstructure:"rate_limit"`
	HiddenTables []string                `mapstructure:"hidden_tables"`
}

type HotReloadConfig struct {
	Enabled            bool          `mapstructure:"enabled"`
	Interval           time.Duration `mapstructure:"interval"`
	AutoNotifyTriggers bool          `mapstructure:"auto_notify_triggers"`
}

type TransactionsConfig struct {
	Enabled         bool          `mapstructure:"enabled"`
	TTL             time.Duration `mapstructure:"ttl"`
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
}

type RateLimitTableConfig struct {
	RPS   float64 `mapstructure:"requests_per_second"`
	Burst int     `mapstructure:"burst"`
}

type RateLimitConfig struct {
	Enabled  bool                             `mapstructure:"enabled"`
	RPS      float64                          `mapstructure:"requests_per_second"`
	Burst    int                              `mapstructure:"burst"`
	PerTable map[string]RateLimitTableConfig  `mapstructure:"per_table"`
}

type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type DatabaseConfig struct {
	Host     string     `mapstructure:"host"`
	Port     int        `mapstructure:"port"`
	User     string     `mapstructure:"user"`
	Password string     `mapstructure:"password"`
	Name     string     `mapstructure:"name"`
	Schemas  []string   `mapstructure:"schemas"`
	SSLMode  string     `mapstructure:"sslmode"`
	Pool     PoolConfig `mapstructure:"pool"`
}

type PoolConfig struct {
	MaxOpen        int           `mapstructure:"max_open"`
	MaxIdle        int           `mapstructure:"max_idle"`
	ConnMaxLife    time.Duration `mapstructure:"conn_max_life"`
	ConnMaxIdle    time.Duration `mapstructure:"conn_max_idle"`
}

type PermConfig struct {
	Methods []string `mapstructure:"methods"`
}

type RPCConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

func Load() *Config {
	v := viper.New()

	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 3000)
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "postgres")
	v.SetDefault("database.password", "postgres")
	v.SetDefault("database.name", "app")
	v.SetDefault("database.schemas", []string{"public"})
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.pool.max_open", 25)
	v.SetDefault("database.pool.max_idle", 5)
	v.SetDefault("database.pool.conn_max_life", "1h")
	v.SetDefault("database.pool.conn_max_idle", "10m")
	v.SetDefault("permissions", map[string]interface{}{
		"*": map[string]interface{}{
			"methods": []string{"*"},
		},
	})
	v.SetDefault("rpc", map[string]interface{}{
		"*": map[string]interface{}{
			"enabled": true,
		},
	})
	v.SetDefault("transactions.enabled", true)
	v.SetDefault("transactions.ttl", "30m")
	v.SetDefault("transactions.cleanup_interval", "60s")
	v.SetDefault("hot_reload.enabled", false)
	v.SetDefault("hot_reload.interval", "30s")
	v.SetDefault("hot_reload.auto_notify_triggers", true)
	v.SetDefault("rate_limit.enabled", false)
	v.SetDefault("rate_limit.requests_per_second", 10)
	v.SetDefault("rate_limit.burst", 20)

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("/etc/rest-trans")
	v.AddConfigPath("$HOME/.config/rest-trans")

	bindLegacyEnvVars(v)
	bindEnvVars(v)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			fmt.Printf("Warning: error reading config file: %v\n", err)
		}
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		fmt.Printf("Warning: error unmarshalling config: %v\n", err)
	}

	return cfg
}

func bindEnvVars(v *viper.Viper) {
	v.SetEnvPrefix("REST")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
}

func bindLegacyEnvVars(v *viper.Viper) {
	mapping := map[string]string{
		"HOST":            "server.host",
		"PORT":            "server.port",
		"DB_HOST":         "database.host",
		"DB_PORT":         "database.port",
		"DB_USER":         "database.user",
		"DB_PASS":         "database.password",
		"DB_PASSWORD":     "database.password",
		"DB_NAME":         "database.name",
		"DB_SCHEMAS":      "database.schemas",
		"DB_SSLMODE":      "database.sslmode",
		"REST_HOST":       "server.host",
		"REST_PORT":       "server.port",
		"REST_DB_HOST":    "database.host",
		"REST_DB_PORT":    "database.port",
		"REST_DB_USER":    "database.user",
		"REST_DB_PASS":    "database.password",
		"REST_DB_PASSWORD": "database.password",
		"REST_DB_NAME":    "database.name",
		"REST_DB_SCHEMAS": "database.schemas",
		"REST_DB_SSLMODE": "database.sslmode",
	}

	for envKey, viperKey := range mapping {
		if val := os.Getenv(envKey); val != "" {
			v.Set(viperKey, castValue(viperKey, val))
		}
	}
}

func castValue(key, val string) interface{} {
	if strings.HasSuffix(key, ".port") || strings.HasSuffix(key, ".max_open") || strings.HasSuffix(key, ".max_idle") {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	if strings.HasSuffix(key, ".schemas") {
		return strings.Split(val, ",")
	}
	return val
}

func (c *Config) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Database.Host, c.Database.Port, c.Database.User,
		c.Database.Password, c.Database.Name, c.Database.SSLMode)
}

func (c *Config) Schemas() []string {
	return c.Database.Schemas
}

func (c *Config) ServerAddr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

func (c *Config) IsMethodAllowed(table, method string) bool {
	if p, ok := c.Permissions[table]; ok {
		return methodsContain(p.Methods, method)
	}
	if p, ok := c.Permissions["*"]; ok {
		return methodsContain(p.Methods, method)
	}
	return true
}

func (c *Config) IsRPCEnabled(funcName string) bool {
	if r, ok := c.RPC[funcName]; ok {
		return r.Enabled
	}
	if r, ok := c.RPC["*"]; ok {
		return r.Enabled
	}
	return true
}

func (c *Config) IsTableHidden(tableName string) bool {
	for _, pattern := range c.HiddenTables {
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(tableName, prefix) {
				return true
			}
		} else if pattern == tableName {
			return true
		}
	}
	return false
}

func methodsContain(methods []string, method string) bool {
	for _, m := range methods {
		if m == "*" || strings.EqualFold(m, "all") || strings.EqualFold(m, method) {
			return true
		}
	}
	return false
}
