package db

import (
	"context"
	"log/slog"
	"time"
)

// queryLogger wraps an slog.Logger with a query timeout and consistent
// op/duration/error logging. Extracted from executor so the bookkeeping
// pieces are independently testable.
type queryLogger struct {
	logger       *slog.Logger
	queryTimeout time.Duration
}

func newQueryLogger(logger *slog.Logger, timeout time.Duration) *queryLogger {
	if logger == nil {
		logger = slog.Default()
	}
	return &queryLogger{logger: logger, queryTimeout: timeout}
}

// withTimeout returns a derived context bounded by queryTimeout, or the
// caller's context unchanged when no timeout is configured. The returned
// CancelFunc is always safe to call.
func (l *queryLogger) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if l.queryTimeout > 0 {
		return context.WithTimeout(ctx, l.queryTimeout)
	}
	return ctx, func() {}
}

// log records a single query attempt. errs are logged at ERROR; successes
// at DEBUG, so noisy debug builds don't pollute production logs.
func (l *queryLogger) log(ctx context.Context, op string, dur time.Duration, err error) {
	attrs := []any{slog.String("op", op), slog.Duration("dur", dur)}
	if err != nil {
		attrs = append(attrs, slog.String("err", err.Error()))
		l.logger.ErrorContext(ctx, "db: query failed", attrs...)
		return
	}
	l.logger.DebugContext(ctx, "db: query ok", attrs...)
}
