package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all application configuration loaded from config.yaml or environment variables.
type Config struct {
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
	v.SetDefault("extensions.repo_url", "https://raw.githubusercontent.com/keiyoushi/extensions/repo/index.min.json")
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

	// Normalize subpath
	if cfg.Server.Subpath != "" && !strings.HasPrefix(cfg.Server.Subpath, "/") {
		cfg.Server.Subpath = "/" + cfg.Server.Subpath
	}
	cfg.Server.Subpath = strings.TrimSuffix(cfg.Server.Subpath, "/")

	return &cfg, nil
}
