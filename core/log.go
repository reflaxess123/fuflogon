package core

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"
)

var (
	logMu     sync.Mutex
	logFile   *os.File
	logInited bool
)

const logFileName = "xray-launcher.log"

// LogsDir returns the Logs subdirectory next to the executable.
func LogsDir(rootDir string) string {
	d := filepath.Join(rootDir, "Logs")
	_ = os.MkdirAll(d, 0755)
	return d
}

// InitLog opens Logs/xray-launcher.log for append. Called once.
func InitLog(rootDir string) {
	logMu.Lock()
	defer logMu.Unlock()
	if logInited {
		return
	}
	logInited = true
	path := filepath.Join(LogsDir(rootDir), logFileName)
	// rotate if too large
	if fi, err := os.Stat(path); err == nil && fi.Size() > 2*1024*1024 {
		os.Rename(path, path+".old")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err == nil {
		logFile = f
	}
}

// Logf writes a line to the log file and stdout.
func Logf(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	line := time.Now().Format("2006-01-02 15:04:05") + " " + msg + "\n"
	logMu.Lock()
	if logFile != nil {
		logFile.WriteString(line)
		logFile.Sync()
	}
	logMu.Unlock()
	fmt.Print(line)
}

// CloseLog closes the log file.
func CloseLog() {
	logMu.Lock()
	defer logMu.Unlock()
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
	logInited = false
}

// RecoverPanic is a defer-helper that catches panics and logs them.
func RecoverPanic(context string) {
	if r := recover(); r != nil {
		stack := string(debug.Stack())
		Logf("[PANIC] in %s: %v\n%s", context, r, stack)
	}
}
