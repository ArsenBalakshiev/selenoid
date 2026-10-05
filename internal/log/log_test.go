package log

import (
	"bytes"
	"log/slog"
	"testing"

	assert "github.com/stretchr/testify/require"
)

func capture(t *testing.T, fn func()) string {
	defer func() { Setup("", "") }()
	var buf bytes.Buffer
	logger.Store(slog.New(NewTextHandler(&buf, slog.LevelDebug)))
	fn()
	return buf.String()
}

func testSetup(format, level string) func() {
	old := logger.Load()
	Setup(format, level)
	return func() { logger.Store(old) }
}

func TestPrintfLegacyFormat(t *testing.T) {
	out := capture(t, func() {
		Printf(42, "SESSION_CREATED", "%s %d", "sid", 1)
	})
	assert.Contains(t, out, "[42] [SESSION_CREATED] [sid 1]")
}

func TestPrintfNoId(t *testing.T) {
	out := capture(t, func() {
		PrintfNoId("INIT", "Hello %s", "world")
	})
	assert.Contains(t, out, "[-] [INIT] [Hello world]")
}

func TestLevelsFiltering(t *testing.T) {
	restore := testSetup("", "warn")
	defer restore()
	var buf bytes.Buffer
	logger.Store(slog.New(NewTextHandler(&buf, slog.LevelWarn)))

	Printf(1, "TAG", "[ignored]")
	PrintfWarn(1, "TAG", "[visible]")
	assert.NotContains(t, buf.String(), "ignored")
	assert.Contains(t, buf.String(), "visible")
}

func TestWarnAndErrorLevels(t *testing.T) {
	var buf bytes.Buffer
	logger.Store(slog.New(NewTextHandler(&buf, slog.LevelDebug)))
	PrintfWarn(1, "WARN_TAG", "%s", "warn-msg")
	PrintfErr(2, "ERR_TAG", "%s", "err-msg")
	Debugf(3, "DBG_TAG", "%s", "dbg-msg")
	out := buf.String()
	assert.Contains(t, out, "[1] [WARN_TAG] [warn-msg]")
	assert.Contains(t, out, "[2] [ERR_TAG] [err-msg]")
	assert.Contains(t, out, "[3] [DBG_TAG] [dbg-msg]")
}

func TestSetupJSON(t *testing.T) {
	restore := testSetup("json", "info")
	defer restore()
	logger.Store(slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)))
}

func TestSetupLevelParsing(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error", "", "unknown"} {
		assert.NotPanics(t, func() { Setup("", level) })
	}
}
