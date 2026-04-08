package core

import (
	"bufio"
	"fmt"
	"io"
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

	// In-memory ring buffer of recent log lines for the UI Logs tab.
	// Capped at maxLogBuffer; oldest entries are dropped first.
	logBuffer    []string
	maxLogBuffer = 5000
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

// Logf writes a line to the log file, stdout and the in-memory buffer.
func Logf(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	line := time.Now().Format("2006-01-02 15:04:05") + " " + msg
	logMu.Lock()
	if logFile != nil {
		logFile.WriteString(line + "\n")
		logFile.Sync()
	}
	logBuffer = append(logBuffer, line)
	if len(logBuffer) > maxLogBuffer {
		// Drop oldest 10% to avoid frequent slice copies on every overflow.
		drop := maxLogBuffer / 10
		logBuffer = append([]string(nil), logBuffer[drop:]...)
	}
	logMu.Unlock()
	fmt.Println(line)
}

// GetLogBuffer returns a snapshot copy of the current in-memory log buffer.
func GetLogBuffer() []string {
	logMu.Lock()
	defer logMu.Unlock()
	out := make([]string, len(logBuffer))
	copy(out, logBuffer)
	return out
}

// ClearLogBuffer empties the in-memory log buffer (does not touch the file).
func ClearLogBuffer() {
	logMu.Lock()
	logBuffer = logBuffer[:0]
	logMu.Unlock()
}

// LogBufferSize returns the current number of lines in the buffer.
func LogBufferSize() int {
	logMu.Lock()
	defer logMu.Unlock()
	return len(logBuffer)
}

// appendBufferLine appends a single pre-formatted line to the buffer without
// writing to the log file. Used by external log tailers (e.g. xray's stderr).
func appendBufferLine(line string) {
	logMu.Lock()
	defer logMu.Unlock()
	logBuffer = append(logBuffer, line)
	if len(logBuffer) > maxLogBuffer {
		drop := maxLogBuffer / 10
		logBuffer = append([]string(nil), logBuffer[drop:]...)
	}
}

// TailXrayLog watches Logs/xray-error.log and pipes any new lines into the
// in-memory log buffer with a [XRAY] prefix. Designed to run in a goroutine
// for the lifetime of the app. Survives file rotation and missing files.
//
// On first start it skips any pre-existing content (we only want logs produced
// during this session — the file is shared across runs and may contain MB of
// stale lines).
func TailXrayLog(rootDir string) {
	path := filepath.Join(LogsDir(rootDir), "xray-error.log")
	var offset int64 = -1 // -1 = "first iteration, jump to end of file"
	for {
		time.Sleep(500 * time.Millisecond)

		f, err := os.Open(path)
		if err != nil {
			// File doesn't exist yet — keep polling.
			offset = 0
			continue
		}
		stat, err := f.Stat()
		if err != nil {
			f.Close()
			continue
		}
		size := stat.Size()
		if offset < 0 {
			// First iteration: skip everything that was already in the file
			// before we started. Only new lines from this session interest us.
			offset = size
			f.Close()
			continue
		}
		if size < offset {
			// File was truncated or rotated → start from beginning.
			offset = 0
		}
		if size == offset {
			f.Close()
			continue
		}

		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			f.Close()
			continue
		}
		scanner := bufio.NewScanner(f)
		// Allow long lines (xray panics print stack traces).
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			appendBufferLine(time.Now().Format("2006-01-02 15:04:05") + " [XRAY] " + line)
		}
		offset = size
		f.Close()
	}
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
