package db

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestNewQueryLogger_NilLoggerUsesDefault(t *testing.T) {
	ql := newQueryLogger(nil, 0)
	if ql.logger == nil {
		t.Fatal("expected non-nil logger when nil provided")
	}
}

func TestQueryLogger_WithTimeout_ZeroIsPassthrough(t *testing.T) {
	ql := newQueryLogger(slog.Default(), 0)
	parent, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()

	ctx, cancel := ql.withTimeout(parent)
	defer cancel()

	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		t.Error("expected no deadline when queryTimeout=0")
	}
	if ctx != parent {
		t.Error("expected parent ctx returned unchanged when queryTimeout=0")
	}
}

func TestQueryLogger_WithTimeout_AppliesDeadline(t *testing.T) {
	ql := newQueryLogger(slog.Default(), 50*time.Millisecond)
	ctx, cancel := ql.withTimeout(context.Background())
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline to be set")
	}
	remaining := time.Until(deadline)
	if remaining <= 0 || remaining > 50*time.Millisecond {
		t.Errorf("unexpected remaining time: %v", remaining)
	}
}

func TestQueryLogger_WithTimeout_ExpiresContext(t *testing.T) {
	ql := newQueryLogger(slog.Default(), 5*time.Millisecond)
	ctx, cancel := ql.withTimeout(context.Background())
	defer cancel()

	<-ctx.Done()
	if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", ctx.Err())
	}
}

func TestQueryLogger_Log_SuccessGoesToDebug(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ql := newQueryLogger(logger, 0)

	ql.log(context.Background(), "select", 12*time.Millisecond, nil)

	out := buf.String()
	if !strings.Contains(out, "level=DEBUG") {
		t.Errorf("expected DEBUG level for success, got: %s", out)
	}
	if !strings.Contains(out, "op=select") {
		t.Errorf("expected op attribute, got: %s", out)
	}
	if !strings.Contains(out, "query ok") {
		t.Errorf("expected success message, got: %s", out)
	}
}

func TestQueryLogger_Log_ErrorGoesToError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ql := newQueryLogger(logger, 0)

	ql.log(context.Background(), "insert", 3*time.Millisecond, errors.New("boom"))

	out := buf.String()
	if !strings.Contains(out, "level=ERROR") {
		t.Errorf("expected ERROR level for failure, got: %s", out)
	}
	if !strings.Contains(out, "op=insert") {
		t.Errorf("expected op attribute, got: %s", out)
	}
	if !strings.Contains(out, "boom") {
		t.Errorf("expected error text in output, got: %s", out)
	}
}

func TestQueryLogger_Log_DebugSuppressedAtInfoLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ql := newQueryLogger(logger, 0)

	ql.log(context.Background(), "select", time.Millisecond, nil)

	if buf.Len() != 0 {
		t.Errorf("expected no log output at INFO level for successful query, got: %s", buf.String())
	}
}

func TestNewExecutor_EmbedsQueryLogger(t *testing.T) {
	e := newExecutor(slog.Default(), 100*time.Millisecond)
	if e.queryLogger == nil {
		t.Fatal("executor.queryLogger is nil")
	}
	if e.queryTimeout != 100*time.Millisecond {
		t.Errorf("queryTimeout: got %v, want 100ms", e.queryTimeout)
	}
}
