package util

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"tsuruyomi/internal/config"
)

type ctxKey string

// ContextLoggerKey is the context key under which the request/task logger is stored.
const ContextLoggerKey ctxKey = "ctx_logger"

var (
	dsnRegex   = regexp.MustCompile(`(postgres(?:ql)?://[^:/\s]+:)(.+?)(@[^/@\s?#]+(?::[0-9]+)?)([/?#]|\s|$)`)
	tokenRegex = regexp.MustCompile(`(?i)(bearer\s+)([a-zA-Z0-9_\-\.]+)`)

	activeBufferedWriter *syncedBufferedWriter
	activeBufferedMu     sync.Mutex
)

type syncedBufferedWriter struct {
	mu     sync.Mutex
	bw     *bufio.Writer
	ticker *time.Ticker
	stopCh chan struct{}
}

func newSyncedBufferedWriter(w io.Writer, size int) *syncedBufferedWriter {
	sbw := &syncedBufferedWriter{
		bw:     bufio.NewWriterSize(w, size),
		ticker: time.NewTicker(250 * time.Millisecond),
		stopCh: make(chan struct{}),
	}

	go func() {
		for {
			select {
			case <-sbw.ticker.C:
				sbw.Flush()
			case <-sbw.stopCh:
				sbw.Flush()
				return
			}
		}
	}()

	return sbw
}

func (s *syncedBufferedWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bw.Write(p)
}

func (s *syncedBufferedWriter) Flush() {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.bw.Flush()
}

func (s *syncedBufferedWriter) Stop() {
	s.ticker.Stop()
	close(s.stopCh)
}

// FlushLogger flushes any pending buffered log lines to stdout.
func FlushLogger() {
	activeBufferedMu.Lock()
	defer activeBufferedMu.Unlock()
	if activeBufferedWriter != nil {
		activeBufferedWriter.Flush()
	}
}

// InitLogger initializes and sets the global default slog.Logger according to LoggingConfig.
func InitLogger(cfg config.LoggingConfig) *slog.Logger {
	activeBufferedMu.Lock()
	if activeBufferedWriter != nil {
		activeBufferedWriter.Stop()
	}
	activeBufferedWriter = newSyncedBufferedWriter(os.Stdout, 32*1024)
	writer := activeBufferedWriter
	activeBufferedMu.Unlock()

	logger, _ := newLoggerWithWriter(cfg, writer)
	slog.SetDefault(logger)
	return logger
}

// InitLoggerWithWriter initializes a logger writing to the provided io.Writer (useful for testing or redirection).
func InitLoggerWithWriter(cfg config.LoggingConfig, w io.Writer) (*slog.Logger, func()) {
	return newLoggerWithWriter(cfg, w)
}

func newLoggerWithWriter(cfg config.LoggingConfig, w io.Writer) (*slog.Logger, func()) {
	var logLevel slog.Level
	switch strings.ToLower(cfg.Level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:       logLevel,
		ReplaceAttr: redactSensitiveAttrs,
	}

	var handler slog.Handler
	if strings.ToLower(cfg.Format) == "json" {
		handler = slog.NewJSONHandler(w, opts)
	} else {
		handler = slog.NewTextHandler(w, opts)
	}

	logger := slog.New(handler)
	cleanup := func() {
		if flusher, ok := w.(interface{ Flush() }); ok {
			flusher.Flush()
		}
	}
	return logger, cleanup
}

// LoggerFromContext retrieves a context-scoped slog.Logger or returns slog.Default().
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return slog.Default()
	}
	if logger, ok := ctx.Value(ContextLoggerKey).(*slog.Logger); ok && logger != nil {
		return logger
	}
	return slog.Default()
}

func sanitizeString(str string) string {
	if dsnRegex.MatchString(str) {
		str = dsnRegex.ReplaceAllString(str, "${1}[REDACTED]${3}${4}")
	}
	if tokenRegex.MatchString(str) {
		str = tokenRegex.ReplaceAllString(str, "${1}[REDACTED]")
	}
	return str
}

func redactSensitiveAttrs(groups []string, a slog.Attr) slog.Attr {
	key := strings.ToLower(a.Key)
	if strings.Contains(key, "password") ||
		strings.Contains(key, "secret") ||
		strings.Contains(key, "cookie") ||
		strings.Contains(key, "authorization") ||
		strings.Contains(key, "token") ||
		strings.Contains(key, "api_key") {
		return slog.String(a.Key, "[REDACTED]")
	}

	if a.Value.Kind() == slog.KindString {
		str := a.Value.String()
		sanitized := sanitizeString(str)
		if sanitized != str {
			return slog.String(a.Key, sanitized)
		}
		return a
	}

	if a.Value.Kind() == slog.KindAny {
		if err, ok := a.Value.Any().(error); ok && err != nil {
			str := err.Error()
			sanitized := sanitizeString(str)
			if sanitized != str {
				return slog.String(a.Key, sanitized)
			}
		}
	}

	return a
}
