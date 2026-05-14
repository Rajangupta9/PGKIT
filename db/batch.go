package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/skolio/pgkit/qb"
)

// Batch accumulates multiple queries and sends them in a single network
// round-trip. Results are returned in the same order queries were added.
type Batch struct {
	entries []batchEntry
	err     error
}

type batchEntry struct {
	sql         string
	args        []any
	returnsRows bool
}

// NewBatch creates an empty Batch.
func NewBatch() *Batch { return &Batch{} }

// addErr stores the first error encountered during building.
func (b *Batch) addErr(err error) *Batch {
	if b.err == nil {
		b.err = err
	}
	return b
}

// Add queues a raw SQL statement that returns rows (e.g. SELECT, or INSERT … RETURNING).
func (b *Batch) Add(sql string, args ...any) *Batch {
	b.entries = append(b.entries, batchEntry{sql: sql, args: args, returnsRows: true})
	return b
}

// AddExec queues a raw SQL statement that does not return rows (e.g. UPDATE, DELETE without RETURNING).
func (b *Batch) AddExec(sql string, args ...any) *Batch {
	b.entries = append(b.entries, batchEntry{sql: sql, args: args, returnsRows: false})
	return b
}

// AddSelect queues a SELECT from a Builder.
func (b *Batch) AddSelect(builder *qb.Builder) *Batch {
	if builder == nil {
		return b.addErr(fmt.Errorf("db/batch: nil builder for Select"))
	}
	sql, args, err := builder.BuildSelect()
	if err != nil {
		return b.addErr(fmt.Errorf("db/batch: build SELECT: %w", err))
	}
	b.entries = append(b.entries, batchEntry{sql: sql, args: args, returnsRows: true})
	return b
}

// AddInsert queues an INSERT from a Builder + data map.
func (b *Batch) AddInsert(builder *qb.Builder, data map[string]any) *Batch {
	if builder == nil {
		return b.addErr(fmt.Errorf("db/batch: nil builder for Insert"))
	}
	sql, args, err := builder.BuildInsert(data)
	if err != nil {
		return b.addErr(fmt.Errorf("db/batch: build INSERT: %w", err))
	}
	b.entries = append(b.entries, batchEntry{sql: sql, args: args, returnsRows: true})
	return b
}

// AddUpdate queues an UPDATE from a Builder + data map.
func (b *Batch) AddUpdate(builder *qb.Builder, data map[string]any) *Batch {
	if builder == nil {
		return b.addErr(fmt.Errorf("db/batch: nil builder for Update"))
	}
	sql, args, err := builder.BuildUpdate(data)
	if err != nil {
		return b.addErr(fmt.Errorf("db/batch: build UPDATE: %w", err))
	}
	b.entries = append(b.entries, batchEntry{sql: sql, args: args, returnsRows: builder.HasReturning()})
	return b
}

// AddDelete queues a DELETE from a Builder.
func (b *Batch) AddDelete(builder *qb.Builder) *Batch {
	if builder == nil {
		return b.addErr(fmt.Errorf("db/batch: nil builder for Delete"))
	}
	sql, args, err := builder.BuildDelete()
	if err != nil {
		return b.addErr(fmt.Errorf("db/batch: build DELETE: %w", err))
	}
	b.entries = append(b.entries, batchEntry{sql: sql, args: args, returnsRows: builder.HasReturning()})
	return b
}

// Len returns the number of queued queries.
func (b *Batch) Len() int { return len(b.entries) }

// SendWrite sends all batched queries over the write pool in one round-trip.
func (c *Client) SendWrite(ctx context.Context, b *Batch) ([][]map[string]any, error) {
	pool, err := c.mustPool("write")
	if err != nil {
		return nil, err
	}
	return sendBatch(ctx, pool, b)
}

// SendRead sends all batched queries over the read pool in one round-trip.
func (c *Client) SendRead(ctx context.Context, b *Batch) ([][]map[string]any, error) {
	pool, err := c.mustPool("read")
	if err != nil {
		return nil, err
	}
	return sendBatch(ctx, pool, b)
}

func sendBatch(ctx context.Context, pool *pgxpool.Pool, b *Batch) ([][]map[string]any, error) {
	if b.err != nil {
		return nil, b.err
	}
	if len(b.entries) == 0 {
		return nil, ErrEmptyRows
	}

	batch := &pgx.Batch{}
	for _, e := range b.entries {
		batch.Queue(e.sql, e.args...)
	}

	br := pool.SendBatch(ctx, batch)
	defer br.Close()

	results := make([][]map[string]any, len(b.entries))
	for i, e := range b.entries {
		if e.returnsRows {
			rows, err := br.Query()
			if err != nil {
				return nil, fmt.Errorf("db/batch: query %d: %w", i, err)
			}
			collected, collectErr := collectRows(rows)
			rows.Close()
			if collectErr != nil {
				return nil, fmt.Errorf("db/batch: collect %d: %w", i, collectErr)
			}
			results[i] = collected
		} else {
			_, err := br.Exec()
			if err != nil {
				return nil, fmt.Errorf("db/batch: exec %d: %w", i, err)
			}
			results[i] = nil
		}
	}

	if err := br.Close(); err != nil {
		return nil, fmt.Errorf("db/batch: close: %w", err)
	}
	return results, nil
}