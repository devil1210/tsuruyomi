# Configuration Management (`internal/config`)

## Overview
The `internal/config` package provides hierarchical configuration loading, environment variable mapping, and dynamic server settings overlay for Tsuruyomi. It reads configuration files via Viper, binds container and OS environment variables, and enables updating and saving configuration state back to YAML persistence.

## Interface
- `[config.go:Config]`: Root application configuration struct containing sub-configurations.
- `[config.go:ServerConfig]`: HTTP server binding settings (`Host`, `Port`, `Subpath`).
- `[config.go:ServerConfig.Addr]`: `func (s ServerConfig) Addr() string` returning formatted `host:port` address.
- `[config.go:DatabaseConfig]`: PostgreSQL connection settings (`DSN`, `MaxConns`, `MinConns`).
- `[config.go:DataConfig]`: Local storage directory paths (`RootDir`, `DownloadsPath`).
- `[config.go:AuthConfig]`: Security credentials and authentication modes (`Mode`, `Username`, `Password`).
- `[config.go:ExtensionsConfig]`: Remote extension repository configuration (`RepoURL`).
- `[config.go:SidecarConfig]`: Optional JVM sidecar proxy configuration (`Enabled`, `BaseURL`).
- `[config.go:FlareSolverrConfig]`: Cloudflare challenge solver settings (`Enabled`, `URL`, `Timeout`).
- `[config.go:LoggingConfig]`: Logging verbosity level and output format configuration (`Level`, `Format`).
- `[config.go:Load]`: `func Load(cfgFile string) (*Config, error)` loading settings from disk and environment.
- `[config.go:Config.ApplyServerSettings]`: `func (c *Config) ApplyServerSettings(settings map[string]string)` applying database settings overrides in memory.
- `[config.go:Config.Save]`: `func (c *Config) Save() error` serializing and saving configuration back to YAML file.

## Invariants
- `[config.go:Load]`: `Server.Subpath` is normalized with a leading slash and trimmed trailing slash when non-empty.
- `[config.go:Load]`: `ConfigPath` defaults to `"config.yaml"` when `cfgFile` parameter is an empty string.
- `[config.go:Load]`: Environment variables `SERVER_PORT` or `PORT` only override `Server.Port` if successfully parsed and value is greater than 0.
- `[config.go:Load]`: Database DSN fallback builds connection string from `DBUSER`, `DBPASS`, `DBHOST`, `DBPORT`, `DBNAME` if `DATABASE_DSN` and `DATABASE_URL` are unset.
- `[config.go:Config.ApplyServerSettings]`: `ApplyServerSettings` returns immediately without changes if receiver `Config` is nil or settings map is empty.
- `[config.go:Config.ApplyServerSettings]`: `flaresolverrtimeout` setting only updates `FlareSolverr.Timeout` if successfully parsed and value is greater than 0.
- `[config.go:Config.ApplyServerSettings]`: `flaresolverrurl` setting only updates `FlareSolverr.URL` if non-empty after trimming whitespace.
- `[config.go:Config.ApplyServerSettings]`: `authmode` setting only updates `Auth.Mode` if non-empty.
- `[config.go:Config.Save]`: `Save` returns nil without writing if receiver `Config` is nil or `ConfigPath` is empty.

## Side Effects
- `[config.go:Load]`: Reads configuration file from disk using Viper (`cfgFile`, `"."`, or `"$HOME/.suwayomi-go"`).
- `[config.go:Load]`: Reads system environment variables (`SUWAYOMI_*`, `DATABASE_DSN`, `DATABASE_URL`, `DBUSER`, `DBPASS`, `DBHOST`, `DBPORT`, `DBNAME`, `SERVER_PORT`, `PORT`, `SIDECAR_ENABLED`, `SIDECAR_BASE_URL`, `FLARESOLVERR_ENABLED`, `FLARESOLVERR_URL`, `LOGGING_FORMAT`).
- `[config.go:Config.ApplyServerSettings]`: Mutates receiver `Config` struct fields in memory based on settings map.
- `[config.go:Config.Save]`: Writes configuration YAML file to disk at `ConfigPath` with file mode `0644` via `os.WriteFile`.

## Verification
- Run: `go test ./...`
