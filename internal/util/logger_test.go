package util

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"tsuruyomi/internal/config"
)

func TestLogger_Redaction_KeyBased(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.LoggingConfig{Level: "info", Format: "json"}
	logger, cleanup := InitLoggerWithWriter(cfg, &buf)
	defer cleanup()

	logger.Info("user login attempt",
		slog.String("password", "supersecret123"),
		slog.String("api_key", "sk_live_123456789"),
		slog.String("user_token", "jwt_token_abc"),
		slog.String("cookie", "session_id=98765"),
		slog.String("authorization", "Basic dXNlcjpwYXNz"),
	)

	output := buf.String()
	var logMap map[string]any
	if err := json.Unmarshal([]byte(output), &logMap); err != nil {
		t.Fatalf("failed to parse json log output: %v. Output: %s", err, output)
	}

	keysToCheck := []string{"password", "api_key", "user_token", "cookie", "authorization"}
	for _, k := range keysToCheck {
		val, ok := logMap[k].(string)
		if !ok {
			t.Errorf("expected string for key %q, got %v", k, logMap[k])
		} else if val != "[REDACTED]" {
			t.Errorf("expected [REDACTED] for key %q, got %q", k, val)
		}
	}
}

func TestLogger_Redaction_ValueBasedDSNAndBearer(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.LoggingConfig{Level: "info", Format: "json"}
	logger, cleanup := InitLoggerWithWriter(cfg, &buf)
	defer cleanup()

	dsnRaw := "postgres://tsuruyomi_user:super_secret_pw@localhost:5432/tsuruyomi_db?sslmode=disable"
	authRaw := "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30.fake_signature"

	logger.Info("database connect failed",
		slog.String("raw_dsn", dsnRaw),
		slog.String("auth_header", authRaw),
	)

	output := buf.String()
	if strings.Contains(output, "super_secret_pw") {
		t.Errorf("log output contains raw database password! Output: %s", output)
	}
	if !strings.Contains(output, "postgres://tsuruyomi_user:[REDACTED]@localhost:5432") {
		t.Errorf("expected redacted DSN in log output. Output: %s", output)
	}

	if strings.Contains(output, "fake_signature") {
		t.Errorf("log output contains raw Bearer token! Output: %s", output)
	}
	if !strings.Contains(output, "Bearer [REDACTED]") {
		t.Errorf("expected redacted Bearer token in log output. Output: %s", output)
	}
}

func TestLogger_Redaction_InErrorValue(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.LoggingConfig{Level: "info", Format: "json"}
	logger, cleanup := InitLoggerWithWriter(cfg, &buf)
	defer cleanup()

	errWithDSN := errors.New("pq: password authentication failed for postgresql://admin:p@ssword!@db.host:5432/manga")
	logger.Error("connection failed", slog.Any("err", errWithDSN))

	output := buf.String()
	if strings.Contains(output, "p@ssword!") {
		t.Errorf("log output leaked password inside error object: %s", output)
	}
	if !strings.Contains(output, "postgresql://admin:[REDACTED]@db.host:5432") {
		t.Errorf("expected redacted DSN inside error attribute: %s", output)
	}
}

func TestLogger_ContextLoggerKey(t *testing.T) {
	// Nil context returns default logger
	lDefault := LoggerFromContext(nil)
	if lDefault == nil {
		t.Fatal("expected non-nil default logger from nil context")
	}

	// Empty context returns default logger
	lEmpty := LoggerFromContext(context.Background())
	if lEmpty == nil {
		t.Fatal("expected non-nil default logger from empty context")
	}

	// Injected logger
	var buf bytes.Buffer
	cfg := config.LoggingConfig{Level: "debug", Format: "text"}
	customLogger, _ := InitLoggerWithWriter(cfg, &buf)

	ctx := context.WithValue(context.Background(), ContextLoggerKey, customLogger)
	retrieved := LoggerFromContext(ctx)
	if retrieved != customLogger {
		t.Errorf("expected retrieved logger to match custom logger from context")
	}
}
