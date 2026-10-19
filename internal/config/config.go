package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all application configuration loaded from config.yaml or environment variables.
type Config struct {
	ConfigPath   string             `mapstructure:"-"`
	Server       ServerConfig       `mapstructure:"server"`
	Database     DatabaseConfig     `mapstructure:"database"`
	Data         DataConfig         `mapstructure:"data"`
	Auth         AuthConfig         `mapstructure:"auth"`
	Extensions   ExtensionsConfig   `mapstructure:"extensions"`
	Logging      LoggingConfig      `mapstructure:"logging"`
	Sidecar      SidecarConfig      `mapstructure:"sidecar"`
	FlareSolverr FlareSolverrConfig `mapstructure:"flaresolverr"`
}

// ServerConfig defines the HTTP server settings.
type ServerConfig struct {
	Host    string `mapstructure:"host"`
	Port    int    `mapstructure:"port"`
	Subpath string `mapstructure:"subpath"`
}

// Addr returns the full listen address string.
func (s ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	DSN      string `mapstructure:"dsn"`
	MaxConns int32  `mapstructure:"max_conns"`
	MinConns int32  `mapstructure:"min_conns"`
}

// DataConfig specifies the root directory for local file storage.
type DataConfig struct {
	RootDir string `mapstructure:"root_dir"`
}

// AuthConfig controls authentication mode.
type AuthConfig struct {
	// Mode: "none", "basic", or "simple_login"
	Mode     string `mapstructure:"mode"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// ExtensionsConfig holds extension-related configuration.
type ExtensionsConfig struct {
	RepoURL string `mapstructure:"repo_url"`
}

// SidecarConfig defines JVM sidecar proxy settings.
type SidecarConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	BaseURL string `mapstructure:"base_url"`
}

// FlareSolverrConfig defines FlareSolverr integration settings.
type FlareSolverrConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	URL     string `mapstructure:"url"`
	Timeout int    `mapstructure:"timeout"`
}

// LoggingConfig defines log verbosity.
type LoggingConfig struct {
	Level string `mapstructure:"level"`
}

// Load reads configuration from the given file path.
// Environment variables prefixed with SUWAYOMI_ override file settings.
// Example: SUWAYOMI_SERVER_PORT=8080
func Load(cfgFile string) (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 4567)
	v.SetDefault("server.subpath", "")
	v.SetDefault("database.dsn", "postgres://tsuruyomi:password@localhost:5432/tsuruyomi?sslmode=disable")
	v.SetDefault("database.max_conns", 20)
	v.SetDefault("database.min_conns", 2)
	v.SetDefault("data.root_dir", "./data")
	v.SetDefault("auth.mode", "none")
	v.SetDefault("extensions.repo_url", "")
	v.SetDefault("logging.level", "info")
	v.SetDefault("sidecar.enabled", false)
	v.SetDefault("sidecar.base_url", "http://127.0.0.1:4568")
	v.SetDefault("flaresolverr.enabled", false)
	v.SetDefault("flaresolverr.url", "http://127.0.0.1:8191")
	v.SetDefault("flaresolverr.timeout", 60)

	// Config file
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("$HOME/.suwayomi-go")
	}

	// Environment overrides
	v.SetEnvPrefix("SUWAYOMI")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config: %w", err)
		}
		// Config file not found is OK — we use defaults + env vars
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	// Standard container / 12-factor environment variable overrides
	if envDSN := os.Getenv("DATABASE_DSN"); envDSN != "" {
		cfg.Database.DSN = envDSN
	} else if envURL := os.Getenv("DATABASE_URL"); envURL != "" {
		cfg.Database.DSN = envURL
	} else if os.Getenv("DBUSER") != "" || os.Getenv("DBHOST") != "" {
		user := os.Getenv("DBUSER")
		if user == "" {
			user = "tsuruyomi"
		}
		pass := os.Getenv("DBPASS")
		if pass == "" {
			pass = "tsuruyomi_pass"
		}
		host := os.Getenv("DBHOST")
		if host == "" {
			host = "postgres"
		}
		port := os.Getenv("DBPORT")
		if port == "" {
			port = "5432"
		}
		dbname := os.Getenv("DBNAME")
		if dbname == "" {
			dbname = "tsuruyomi"
		}
		cfg.Database.DSN = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, port, dbname)
	}

	if envPort := os.Getenv("SERVER_PORT"); envPort != "" {
		var p int
		if _, err := fmt.Sscan(envPort, &p); err == nil && p > 0 {
			cfg.Server.Port = p
		}
	} else if envPort := os.Getenv("PORT"); envPort != "" {
		var p int
		if _, err := fmt.Sscan(envPort, &p); err == nil && p > 0 {
			cfg.Server.Port = p
		}
	}

	if envSidecar := os.Getenv("SIDECAR_ENABLED"); envSidecar != "" {
		cfg.Sidecar.Enabled = (envSidecar == "true" || envSidecar == "1")
	}
	if envSidecarURL := os.Getenv("SIDECAR_BASE_URL"); envSidecarURL != "" {
		cfg.Sidecar.BaseURL = envSidecarURL
	}
	if envFlare := os.Getenv("FLARESOLVERR_ENABLED"); envFlare != "" {
		cfg.FlareSolverr.Enabled = (envFlare == "true" || envFlare == "1")
	}
	if envFlareURL := os.Getenv("FLARESOLVERR_URL"); envFlareURL != "" {
		cfg.FlareSolverr.URL = envFlareURL
	}

	// Normalize subpath
	if cfg.Server.Subpath != "" && !strings.HasPrefix(cfg.Server.Subpath, "/") {
		cfg.Server.Subpath = "/" + cfg.Server.Subpath
	}
	cfg.Server.Subpath = strings.TrimSuffix(cfg.Server.Subpath, "/")

	cfg.ConfigPath = cfgFile
	if cfg.ConfigPath == "" {
		cfg.ConfigPath = "config.yaml"
	}

	return &cfg, nil
}

// ApplyServerSettings updates the in-memory Config with key-value settings from the database.
func (c *Config) ApplyServerSettings(settings map[string]string) {
	if c == nil || len(settings) == 0 {
		return
	}
	if v, ok := settings["flaresolverr.enabled"]; ok {
		c.FlareSolverr.Enabled = (v == "true" || v == "1")
	}
	if v, ok := settings["flaresolverr.url"]; ok && strings.TrimSpace(v) != "" {
		c.FlareSolverr.URL = strings.TrimSpace(v)
	}
	if v, ok := settings["flaresolverr.timeout"]; ok {
		var t int
		if _, err := fmt.Sscan(v, &t); err == nil && t > 0 {
			c.FlareSolverr.Timeout = t
		}
	}
	if v, ok := settings["auth.mode"]; ok && v != "" {
		c.Auth.Mode = v
	}
	if v, ok := settings["auth.username"]; ok {
		c.Auth.Username = v
	}
	if v, ok := settings["auth.password"]; ok {
		c.Auth.Password = v
	}
}

// Save attempts to serialize the current config back to config.yaml if writable.
func (c *Config) Save() error {
	if c == nil || c.ConfigPath == "" {
		return nil
	}
	yamlContent := fmt.Sprintf(`# Tsuruyomi Server Configuration
server:
  host: "%s"
  port: %d
  subpath: "%s"

database:
  dsn: "%s"
  max_conns: %d
  min_conns: %d

data:
  root_dir: "%s"

auth:
  mode: "%s"
  username: "%s"
  password: "%s"

extensions:
  repo_url: "%s"

logging:
  level: "%s"

sidecar:
  enabled: %t
  base_url: "%s"

flaresolverr:
  enabled: %t
  url: "%s"
  timeout: %d
`,
		c.Server.Host, c.Server.Port, c.Server.Subpath,
		c.Database.DSN, c.Database.MaxConns, c.Database.MinConns,
		c.Data.RootDir,
		c.Auth.Mode, c.Auth.Username, c.Auth.Password,
		c.Extensions.RepoURL,
		c.Logging.Level,
		c.Sidecar.Enabled, c.Sidecar.BaseURL,
		c.FlareSolverr.Enabled, c.FlareSolverr.URL, c.FlareSolverr.Timeout,
	)

	return os.WriteFile(c.ConfigPath, []byte(yamlContent), 0644)
}

