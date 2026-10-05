// Package log provides structured logging for Selenoid. Default output keeps
// the legacy console format "[<requestId>] [<TAG>] [message]"; when the
// LOG_FORMAT=json environment variable is set, records are emitted as JSON
// objects. Log level is controlled with LOG_LEVEL (debug|info|warn|error).
package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var logger atomic.Pointer[slog.Logger]

func init() {
	Setup(os.Getenv("LOG_FORMAT"), os.Getenv("LOG_LEVEL"))
}

// Setup re-initializes the logger. format "json" switches to structured JSON
// output; anything else keeps the human-readable default. level is one of
// debug, info, warn, error (case-insensitive); empty means info.
func Setup(format, level string) {
	var l slog.Level
	switch strings.ToLower(level) {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: l}
	var h slog.Handler
	if strings.EqualFold(format, "json") {
		h = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		h = NewTextHandler(os.Stdout, l)
	}
	logger.Store(slog.New(h))
}

// L returns the underlying slog logger.
func L() *slog.Logger {
	return logger.Load()
}

func newRecord(level slog.Level, msg string, requestId uint64, tag string, hasId bool) slog.Record {
	r := slog.NewRecord(time.Now(), level, msg, 0)
	if hasId {
		r.AddAttrs(slog.Uint64("requestId", requestId))
	}
	r.AddAttrs(slog.String("tag", tag))
	return r
}

func pass(level slog.Level, r slog.Record) {
	l := logger.Load()
	if l.Handler().Enabled(context.Background(), level) {
		_ = l.Handler().Handle(context.Background(), r)
	}
}

// Printf logs a message preserving the legacy "[%d] [TAG] [data...]" signature:
// the first argument is the request id, the second one the well-known log tag.
func Printf(requestId uint64, tag string, format string, args ...interface{}) {
	pass(slog.LevelInfo, newRecord(slog.LevelInfo, fmt.Sprintf(format, args...), requestId, tag, true))
}

// PrintfWarn behaves like Printf but logs at WARN level.
func PrintfWarn(requestId uint64, tag string, format string, args ...interface{}) {
	pass(slog.LevelWarn, newRecord(slog.LevelWarn, fmt.Sprintf(format, args...), requestId, tag, true))
}

// PrintfErr behaves like Printf but logs at ERROR level.
func PrintfErr(requestId uint64, tag string, format string, args ...interface{}) {
	pass(slog.LevelError, newRecord(slog.LevelError, fmt.Sprintf(format, args...), requestId, tag, true))
}

// Debugf logs at DEBUG level.
func Debugf(requestId uint64, tag string, format string, args ...interface{}) {
	pass(slog.LevelDebug, newRecord(slog.LevelDebug, fmt.Sprintf(format, args...), requestId, tag, true))
}

// PrintfNoId logs init-time messages that have no request id.
func PrintfNoId(tag string, format string, args ...interface{}) {
	pass(slog.LevelInfo, newRecord(slog.LevelInfo, fmt.Sprintf(format, args...), 0, tag, false))
}

// Println logs an untagged, id-less message at INFO level.
func Println(args ...interface{}) {
	pass(slog.LevelInfo, newRecord(slog.LevelInfo, fmt.Sprint(args...), 0, "INIT", false))
}

// Fatalf logs a fatal message and exits with code 1.
func Fatalf(format string, args ...interface{}) {
	FatalNoId("INIT", format, args...)
}

// FatalNoId logs a fatal init-time message (no request id) and exits with code 1.
func FatalNoId(tag string, format string, args ...interface{}) {
	pass(slog.LevelError, newRecord(slog.LevelError, fmt.Sprintf(format, args...), 0, tag, false))
	os.Exit(1)
}

// textHandler renders records in the legacy selenoid format:
//
//	[<requestId>] [<TAG>] [<message>]
//
// Records without a requestId are rendered as "[-] [<TAG>] [<message>]".
type textHandler struct {
	mu    *sync.Mutex
	w     io.Writer
	level slog.Leveler
}

// NewTextHandler creates a handler rendering the legacy console format.
func NewTextHandler(w io.Writer, level slog.Leveler) *textHandler {
	return &textHandler{w: w, level: level, mu: &sync.Mutex{}}
}

func (h *textHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *textHandler) Handle(_ context.Context, r slog.Record) error {
	var sb strings.Builder
	requestId := uint64(0)
	hasId := false
	tag := ""
	r.Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case "requestId":
			requestId = a.Value.Uint64()
			hasId = true
		case "tag":
			tag = a.Value.String()
		}
		return true
	})
	if hasId {
		sb.WriteString("[" + strconv.FormatUint(requestId, 10) + "]")
	} else {
		sb.WriteString("[-]")
	}
	if tag != "" {
		sb.WriteString(" [" + tag + "]")
	}
	sb.WriteString(" [" + r.Message + "]\n")

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, sb.String())
	return err
}

func (h *textHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }

func (h *textHandler) WithGroup(_ string) slog.Handler { return h }
