package logger

import (
	"context"
	"fmt"
	"sync"

	"github.com/luevano/libmangal/progress"
)

// ProviderLogger is the logger of a provider: it logs through the libmangal Logger with the
// provider ID as prefix and reports statuses with the provider name.
//
// The Logger is set later by libmangal (see SetLogger), messages logged before that are kept and
// logged once it is. A nil ProviderLogger (e.g. in tests) does nothing.
type ProviderLogger struct {
	mu      sync.Mutex
	id      string
	name    string
	logger  *Logger
	backlog []string
}

// NewProviderLogger returns the logger of the provider with the given id and name; the name
// falls back to the id when empty.
func NewProviderLogger(id, name string) *ProviderLogger {
	if name == "" {
		name = id
	}
	return &ProviderLogger{id: id, name: name}
}

// SetLogger sets the libmangal Logger and logs the messages kept until now.
func (l *ProviderLogger) SetLogger(logger *Logger) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	l.logger = logger
	if logger == nil {
		return
	}
	for _, msg := range l.backlog {
		logger.Log("[backlog] %s", msg)
	}
	l.backlog = nil
}

// Log logs the message prefixed with "<id>: ".
func (l *ProviderLogger) Log(format string, a ...any) {
	if l == nil {
		return
	}
	msg := l.id + ": " + fmt.Sprintf(format, a...)

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.logger == nil {
		l.backlog = append(l.backlog, msg)
		return
	}
	l.logger.Log("%s", msg)
}

// Status logs the message and reports it, prefixed with "<name>: ", to the progress reporter of
// ctx, for what the user waits on: mirror switches, challenges, rate limits.
func (l *ProviderLogger) Status(ctx context.Context, format string, a ...any) {
	if l == nil {
		return
	}
	l.Log(format, a...)
	progress.Report(ctx, progress.Status{Source: l.id, Message: l.name + ": " + fmt.Sprintf(format, a...)})
}

// Progress reports a routine step (e.g. a search page, see progress.Status.Minor) without
// logging it.
func (l *ProviderLogger) Progress(ctx context.Context, done, total int, format string, a ...any) {
	if l == nil {
		return
	}
	progress.Report(ctx, progress.Status{Source: l.id, Message: l.name + ": " + fmt.Sprintf(format, a...), Done: done, Total: total, Minor: true})
}
