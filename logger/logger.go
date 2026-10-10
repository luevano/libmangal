package logger

import (
	"fmt"
	"io"
	"log"
	"sync"
)

type Logger struct {
	mu     sync.Mutex
	onLog  func(format string, a ...any)
	logger *log.Logger
	prefix string
}

func NewLogger() *Logger {
	return &Logger{
		logger: log.New(io.Discard, "", log.Default().Flags()),
	}
}

// SetPrefix sets the prefix added to every message, followed by ": ".
func (l *Logger) SetPrefix(prefix string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prefix = prefix
}

func (l *Logger) GetPrefix() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.prefix
}

func (l *Logger) Writer() io.Writer {
	return l.logger.Writer()
}

func (l *Logger) SetOutput(writer io.Writer) {
	l.logger.SetOutput(writer)
}

// SetOnLog sets a hook called with every message, its format already prefixed.
//
// It can be called from several goroutines at once.
func (l *Logger) SetOnLog(hook func(format string, a ...any)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.onLog = hook
}

func (l *Logger) Log(format string, a ...any) {
	l.mu.Lock()
	hook, prefix := l.onLog, l.prefix
	l.mu.Unlock()
	if prefix != "" {
		prefix += ": "
	}

	if hook != nil {
		hook(prefix+format, a...)
	}
	l.logger.Print(prefix + fmt.Sprintf(format, a...))
}
