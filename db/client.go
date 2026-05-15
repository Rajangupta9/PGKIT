package db

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rajangupta9/pgkit/qb"
)

// Config holds executor-level settings shared across all pools.
type Config struct {
	// QueryTimeout is applied per-query via context. 0 = no limit.
	QueryTimeout time.Duration

	// Logger is used for query debug/error logging.
	// Defaults to slog.Default() if nil.
	Logger *slog.Logger
}

// Client owns one or more named connection pools and exposes a
// query builder and execution API.
//
// Create once at startup; safe for concurrent use across goroutines.
type Client struct {
	pools *poolManager
	exec  *executor
}

// New creates a Client and establishes all provided named pools.
// At least one NamedPool is required. Each pool can have independent
// credentials, DSN, and sizing. Cancel ctx to abort connection attempts.
func New(ctx context.Context, cfg Config, pools ...NamedPool) (*Client, error) {
	pm, err := newPoolManager(ctx, pools)
	if err != nil {
		return nil, fmt.Errorf("db.New: %w", err)
	}
	return &Client{
		pools: pm,
		exec:  newExecutor(cfg.Logger, cfg.QueryTimeout),
	}, nil
}

// Pool returns the *pgxpool.Pool registered under name.
// Returns nil if no pool with that name was registered.
func (c *Client) Pool(name string) *pgxpool.Pool {
	return c.pools.Get(name)
}

// mustPool returns the pool or an error if not registered.
func (c *Client) mustPool(name string) (*pgxpool.Pool, error) {
	p := c.pools.Get(name)
	if p == nil {
		return nil, fmt.Errorf("db: pool %q is not registered", name)
	}
	return p, nil
}

// ─── Query builder ────────────────────────────────────────────────────────────

// QB returns a new qb.Builder targeting table.
func (c *Client) QB(table string) *qb.Builder {
	return qb.New(table)
}

// ─── Read operations ──────────────────────────────────────────────────────────

// Query executes b.BuildSelect() on the "read" pool and returns all rows.
func (c *Client) Query(ctx context.Context, b *qb.Builder) ([]map[string]any, error) {
	sql, args, err := b.BuildSelect()
	if err != nil {
		return nil, fmt.Errorf("db: build SELECT: %w", err)
	}
	pool, err := c.mustPool("read")
	if err != nil {
		return nil, err
	}
	return c.exec.ExecSelect(ctx, pool, sql, args)
}

// QueryOne executes b.BuildSelect() on the "read" pool and returns the first row.
// Returns ErrNoRows if no rows match. Never mutates the supplied builder.
func (c *Client) QueryOne(ctx context.Context, b *qb.Builder) (map[string]any, error) {
	b2 := b.Clone()
	if b2 == nil {
		return nil, fmt.Errorf("db: nil builder")
	}
	b2.Limit(1)
	sql, args, err := b2.BuildSelect()
	if err != nil {
		return nil, fmt.Errorf("db: build SELECT: %w", err)
	}
	pool, err := c.mustPool("read")
	if err != nil {
		return nil, err
	}
	return c.exec.ExecSelectOne(ctx, pool, sql, args)
}

// QueryWrite executes a SELECT on the "write" pool.
// Use immediately after a write when read replicas may lag.
func (c *Client) QueryWrite(ctx context.Context, b *qb.Builder) ([]map[string]any, error) {
	sql, args, err := b.BuildSelect()
	if err != nil {
		return nil, fmt.Errorf("db: build SELECT: %w", err)
	}
	pool, err := c.mustPool("write")
	if err != nil {
		return nil, err
	}
	return c.exec.ExecSelect(ctx, pool, sql, args)
}

// QueryPool executes b.BuildSelect() on the named pool and returns all rows.
// Use when you need to target a specific pool by name.
func (c *Client) QueryPool(ctx context.Context, poolName string, b *qb.Builder) ([]map[string]any, error) {
	sql, args, err := b.BuildSelect()
	if err != nil {
		return nil, fmt.Errorf("db: build SELECT: %w", err)
	}
	pool, err := c.mustPool(poolName)
	if err != nil {
		return nil, err
	}
	return c.exec.ExecSelect(ctx, pool, sql, args)
}

// ─── Write operations ─────────────────────────────────────────────────────────

// Insert executes b.BuildInsert(data) on the "write" pool and returns the generated UUID from
// RETURNING "id".
func (c *Client) Insert(ctx context.Context, b *qb.Builder, data map[string]any) (uuid.UUID, error) {
	sql, args, err := b.BuildInsert(data)
	if err != nil {
		return uuid.Nil, fmt.Errorf("db: build INSERT: %w", err)
	}
	pool, err := c.mustPool("write")
	if err != nil {
		return uuid.Nil, err
	}
	return c.exec.ExecInsert(ctx, pool, sql, args)
}

// InsertBatch executes b.BuildInsertBatch(rows) on the "write" pool.
// Returns the first generated UUID from RETURNING "id".
func (c *Client) InsertBatch(ctx context.Context, b *qb.Builder, rows []map[string]any) (uuid.UUID, error) {
	sql, args, err := b.BuildInsertBatch(rows)
	if err != nil {
		return uuid.Nil, fmt.Errorf("db: build INSERT BATCH: %w", err)
	}
	pool, err := c.mustPool("write")
	if err != nil {
		return uuid.Nil, err
	}
	return c.exec.ExecInsert(ctx, pool, sql, args)
}

// Update executes b.BuildUpdate(data) on the "write" pool and returns rows affected.
func (c *Client) Update(ctx context.Context, b *qb.Builder, data map[string]any) (int64, error) {
	sql, args, err := b.BuildUpdate(data)
	if err != nil {
		return 0, fmt.Errorf("db: build UPDATE: %w", err)
	}
	pool, err := c.mustPool("write")
	if err != nil {
		return 0, err
	}
	return c.exec.ExecWrite(ctx, pool, sql, args)
}

// Delete executes b.BuildDelete() on the "write" pool and returns rows affected.
func (c *Client) Delete(ctx context.Context, b *qb.Builder) (int64, error) {
	sql, args, err := b.BuildDelete()
	if err != nil {
		return 0, fmt.Errorf("db: build DELETE: %w", err)
	}
	pool, err := c.mustPool("write")
	if err != nil {
		return 0, err
	}
	return c.exec.ExecWrite(ctx, pool, sql, args)
}

// ─── Raw SQL ──────────────────────────────────────────────────────────────────

// ExecSQL executes a raw write statement on the "write" pool.
func (c *Client) ExecSQL(ctx context.Context, sql string, args ...any) (int64, error) {
	pool, err := c.mustPool("write")
	if err != nil {
		return 0, err
	}
	return c.exec.ExecWrite(ctx, pool, sql, args)
}

// ExecPoolSQL executes a raw write statement on the named pool.
func (c *Client) ExecPoolSQL(ctx context.Context, poolName string, sql string, args ...any) (int64, error) {
	pool, err := c.mustPool(poolName)
	if err != nil {
		return 0, err
	}
	return c.exec.ExecWrite(ctx, pool, sql, args)
}

// QuerySQL executes a raw SELECT on the "read" pool and returns all rows.
func (c *Client) QuerySQL(ctx context.Context, sql string, args ...any) ([]map[string]any, error) {
	pool, err := c.mustPool("read")
	if err != nil {
		return nil, err
	}
	return c.exec.ExecSelect(ctx, pool, sql, args)
}

// ─── Lifecycle ────────────────────────────────────────────────────────────────

// HealthCheck pings all registered pools concurrently.
func (c *Client) HealthCheck(ctx context.Context) error {
	return c.pools.HealthCheck(ctx)
}

// Close closes all pools. Call during graceful shutdown.
func (c *Client) Close() {
	c.pools.Close()
}
