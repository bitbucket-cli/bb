package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

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
	ts := time.Now().Format("2006-01-02T15:04:05.000Z07:00")
	fmt.Fprintf(f, "%s pid=%d %s\n", ts, os.Getpid(), fmt.Sprintf(format, args...))
}
