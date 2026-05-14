package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rajangupta9/pgkit/qb"
)

// TxOptions mirrors pgx.TxOptions for caller convenience.
type TxOptions = pgx.TxOptions

// Tx is the transaction API exposed to WithTx callbacks.
type Tx interface {
	// QB returns a new query builder — same as Client.QB.
	QB(table string) *qb.Builder

	// Insert executes INSERT … RETURNING "id" and returns the generated UUID.
	Insert(ctx context.Context, b *qb.Builder, data map[string]any) (uuid.UUID, error)

	// Update executes UPDATE and returns rows affected.
	Update(ctx context.Context, b *qb.Builder, data map[string]any) (int64, error)

	// Delete executes DELETE and returns rows affected.
	Delete(ctx context.Context, b *qb.Builder) (int64, error)

	// Select executes SELECT and returns all rows as []map[string]any.
	Select(ctx context.Context, b *qb.Builder) ([]map[string]any, error)

	// SelectOne executes SELECT and returns the first row or ErrNoRows.
	SelectOne(ctx context.Context, b *qb.Builder) (map[string]any, error)

	// ExecRaw executes a raw write statement and returns rows affected.
	ExecRaw(ctx context.Context, sql string, args ...any) (int64, error)

	// QueryRaw executes a raw SELECT statement and returns all rows.
	QueryRaw(ctx context.Context, sql string, args ...any) ([]map[string]any, error)

	// Savepoint creates a named savepoint within the transaction.
	Savepoint(ctx context.Context, name string) error

	// RollbackTo rolls back to a named savepoint.
	RollbackTo(ctx context.Context, name string) error

	// ReleaseSavepoint releases a named savepoint.
	ReleaseSavepoint(ctx context.Context, name string) error
}

type txImpl struct {
	tx   pgx.Tx
	exec *executor
}

func (t *txImpl) QB(table string) *qb.Builder { return qb.New(table) }

func (t *txImpl) Insert(ctx context.Context, b *qb.Builder, data map[string]any) (uuid.UUID, error) {
	sql, args, err := b.BuildInsert(data)
	if err != nil {
		return uuid.Nil, fmt.Errorf("db/tx: build INSERT: %w", err)
	}
	return t.exec.ExecTxInsert(ctx, t.tx, sql, args)
}

func (t *txImpl) Update(ctx context.Context, b *qb.Builder, data map[string]any) (int64, error) {
	sql, args, err := b.BuildUpdate(data)
	if err != nil {
		return 0, fmt.Errorf("db/tx: build UPDATE: %w", err)
	}
	return t.exec.ExecTxWrite(ctx, t.tx, sql, args)
}

func (t *txImpl) Delete(ctx context.Context, b *qb.Builder) (int64, error) {
	sql, args, err := b.BuildDelete()
	if err != nil {
		return 0, fmt.Errorf("db/tx: build DELETE: %w", err)
	}
	return t.exec.ExecTxWrite(ctx, t.tx, sql, args)
}

func (t *txImpl) Select(ctx context.Context, b *qb.Builder) ([]map[string]any, error) {
	sql, args, err := b.BuildSelect()
	if err != nil {
		return nil, fmt.Errorf("db/tx: build SELECT: %w", err)
	}
	return t.exec.ExecTxSelect(ctx, t.tx, sql, args)
}

func (t *txImpl) SelectOne(ctx context.Context, b *qb.Builder) (map[string]any, error) {
	b2 := b.Clone()
	if b2 == nil {
		return nil, fmt.Errorf("db/tx: nil builder")
	}
	rows, err := t.Select(ctx, b2.Limit(1))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrNoRows
	}
	return rows[0], nil
}

func (t *txImpl) ExecRaw(ctx context.Context, sql string, args ...any) (int64, error) {
	return t.exec.ExecTxWrite(ctx, t.tx, sql, args)
}

func (t *txImpl) QueryRaw(ctx context.Context, sql string, args ...any) ([]map[string]any, error) {
	return t.exec.ExecTxSelect(ctx, t.tx, sql, args)
}

func (t *txImpl) Savepoint(ctx context.Context, name string) error {
	_, err := t.tx.Exec(ctx, "SAVEPOINT "+qb.QuoteIdent(name))
	return err
}

func (t *txImpl) RollbackTo(ctx context.Context, name string) error {
	_, err := t.tx.Exec(ctx, "ROLLBACK TO SAVEPOINT "+qb.QuoteIdent(name))
	return err
}

func (t *txImpl) ReleaseSavepoint(ctx context.Context, name string) error {
	_, err := t.tx.Exec(ctx, "RELEASE SAVEPOINT "+qb.QuoteIdent(name))
	return err
}

// WithTx runs fn inside a transaction on the "write" pool.
// Commits on nil return, rolls back otherwise.
//
//	err := client.WithTx(ctx, func(tx db.Tx) error {
//	    id, err := tx.Insert(ctx, tx.QB("orders"), data)
//	    return err
//	})
func (c *Client) WithTx(ctx context.Context, fn func(Tx) error) error {
	return c.WithTxOpts(ctx, pgx.TxOptions{}, fn)
}

// WithTxOpts is like WithTx but accepts custom transaction options
// (isolation level, access mode).
func (c *Client) WithTxOpts(ctx context.Context, opts TxOptions, fn func(Tx) error) error {
	pool, err := c.mustPool("write")
	if err != nil {
		return err
	}
	return withTx(ctx, pool, c.exec, opts, fn)
}

// WithPoolTx runs fn in a transaction on the named pool.
func (c *Client) WithPoolTx(ctx context.Context, poolName string, fn func(Tx) error) error {
	pool, err := c.mustPool(poolName)
	if err != nil {
		return err
	}
	return withTx(ctx, pool, c.exec, pgx.TxOptions{}, fn)
}

// WithRetryTx runs fn in a SERIALIZABLE transaction on the "write" pool and retries
// automatically on serialization failures (40001). maxRetries caps the attempt count.
// Includes exponential backoff between retries.
func (c *Client) WithRetryTx(ctx context.Context, maxRetries int, fn func(Tx) error) error {
	opts := TxOptions{IsoLevel: pgx.Serializable}
	pool, err := c.mustPool("write")
	if err != nil {
		return err
	}
	var lastErr error
	for i := range maxRetries {
		if i > 0 {
			// Exponential backoff: 10ms, 20ms, 40ms... but honour ctx so a
			// cancelled caller doesn't sit through the remaining delay.
			backoff := time.Duration(1<<(i-1)) * 10 * time.Millisecond
			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
		err := withTx(ctx, pool, c.exec, opts, fn)
		if err == nil {
			return nil
		}
		if IsSerializationFailure(err) {
			lastErr = err
			continue
		}
		return err
	}
	return fmt.Errorf("db: serializable tx failed after %d retries: %w", maxRetries, lastErr)
}

func withTx(ctx context.Context, pool *pgxpool.Pool, exec *executor, opts TxOptions, fn func(Tx) error) error {
	tx, err := pool.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("db: begin tx: %w", err)
	}
	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback(ctx)
			panic(r) // re-panic after rollback so caller still sees the panic
		}
	}()
	if fnErr := fn(&txImpl{tx: tx, exec: exec}); fnErr != nil {
		_ = tx.Rollback(ctx)
		return fnErr
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("db: commit tx: %w", err)
	}
	return nil
}

// AcquireAdvisoryLock acquires a transaction-scoped advisory lock.
// Blocks until the lock is available. Released automatically when the tx ends.
//
//	client.WithTx(ctx, func(tx db.Tx) error {
//	    return db.AcquireAdvisoryLock(ctx, tx, userID)
//	})
func AcquireAdvisoryLock(ctx context.Context, tx Tx, key int64) error {
	_, err := tx.ExecRaw(ctx, "SELECT pg_advisory_xact_lock($1)", key)
	if err != nil {
		return fmt.Errorf("db: advisory lock %d: %w", key, err)
	}
	return nil
}

// TryAdvisoryLock tries to acquire a transaction-scoped advisory lock without
// blocking. Returns true if acquired, false if already held by another session.
// Must be called inside a WithTx callback; the lock is released when the tx ends.
func TryAdvisoryLock(ctx context.Context, tx Tx, key int64) (bool, error) {
	rows, err := tx.QueryRaw(ctx, "SELECT pg_try_advisory_xact_lock($1) AS acquired", key)
	if err != nil {
		return false, fmt.Errorf("db: try advisory lock %d: %w", key, err)
	}
	if len(rows) == 0 {
		return false, nil
	}
	acquired, _ := rows[0]["acquired"].(bool)
	return acquired, nil
}

var _ Tx = (*txImpl)(nil)
