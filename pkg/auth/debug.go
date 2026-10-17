package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/zalando/go-keyring"
)

// Persistent diagnostics classify errors without storing backend messages or
// response bodies, which may contain credential values supplied by a provider.
func debugError(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, keyring.ErrNotFound):
		return "not-found"
	case errors.Is(err, os.ErrNotExist):
		return "file-not-found"
	case errors.Is(err, os.ErrPermission):
		return "permission-denied"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "cancelled"
	default:
		return fmt.Sprintf("backend-error(%T)", err)
	}
}

// DebugLogPath is where token-operation traces are written when BB_AUTH_DEBUG is set.
func DebugLogPath() string {
	home, err := userHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "bb", "auth-debug.log")
}

// debugEnabled reports whether token-operation debug logging is on.
func debugEnabled() bool {
	return os.Getenv("BB_AUTH_DEBUG") != ""
}

// debugLog appends a timestamped, pid-tagged line to the auth debug log when
// BB_AUTH_DEBUG is set. Best-effort — it never affects the caller. The pid tag
// makes concurrent bb invocations (e.g. multiple terminals) distinguishable.
func debugLog(format string, args ...interface{}) {
	if !debugEnabled() {
		return
	}
	path := DebugLogPath()
	if path == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0700)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	if err := f.Chmod(0600); err != nil {
		return
	}
	ts := time.Now().Format("2006-01-02T15:04:05.000Z07:00")
	fmt.Fprintf(f, "%s pid=%d %s\n", ts, os.Getpid(), fmt.Sprintf(format, args...))
}
