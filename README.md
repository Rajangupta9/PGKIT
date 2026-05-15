# PGKIT

[![Go Reference](https://pkg.go.dev/badge/github.com/rajangupta9/pgkit.svg)](https://pkg.go.dev/github.com/rajangupta9/pgkit)
[![Go Report Card](https://goreportcard.com/badge/github.com/rajangupta9/pgkit)](https://goreportcard.com/report/github.com/rajangupta9/pgkit)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![GitHub release](https://img.shields.io/github/v/release/rajangupta9/pgkit)](https://github.com/rajangupta9/pgkit/releases)

Production-grade PostgreSQL toolkit for Go.

PGKIT combines a standalone SQL query builder with a lightweight `pgx`-based database layer for connection pooling, transactions, typed scanning, and PostgreSQL-specific features. It is designed for production workloads with minimal ceremony and a small, idiomatic API surface.

## Features

- **`qb`**: PostgreSQL-native query builder with safe identifier quoting and parameter binding
- **`db`**: Named pool management, query execution, transaction helpers, and typed scan helpers
- `ON CONFLICT` upserts, CTEs, `UNION` / `UNION ALL`, window functions, and row locking
- Typed scanning into Go structs using generics (`QueryInto`, `InsertInto`, etc.)
- `LISTEN` / `NOTIFY` support with dedicated connection handling
- Transaction-scoped advisory locks (`AcquireAdvisoryLock`, `TryAdvisoryLock`)
- Retry loop for `SERIALIZABLE` transactions (`WithRetryTx`)
- Batch query dispatch in a single network round-trip (`SendWrite`, `SendRead`)
- Query timeouts and structured logging via `log/slog`
- IPv4-preferring DNS resolver for cloud environments (GCP Cloud Run, etc.)

## Installation

```bash
go get github.com/rajangupta9/pgkit
```

Requires **Go 1.23** or later.

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/rajangupta9/pgkit/db"
    "github.com/rajangupta9/pgkit/qb"
)

func main() {
    ctx := context.Background()

    client, err := db.New(ctx, db.Config{},
        db.NamedPool{Name: "write", PoolConfig: db.PoolConfig{ConnString: os.Getenv("DATABASE_URL")}},
        db.NamedPool{Name: "read",  PoolConfig: db.PoolConfig{ConnString: os.Getenv("DATABASE_URL")}},
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    rows, err := client.Query(ctx, client.QB("users").
        Columns("id", "name", "email").
        Where(qb.Where("active", qb.OpEq, true)).
        OrderBy("created_at", qb.Desc).
        Limit(10),
    )
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(rows)
}
```

## Query Builder (`qb`)

The `qb` package is a self-contained query builder with no database connection dependency:

```go
import "github.com/rajangupta9/pgkit/qb"

// SELECT
sql, args, err := qb.New("products").
    Columns("id", "name", "price").
    Where(qb.Where("price", qb.OpGt, 100)).
    Where(qb.WhereIn("category", []string{"books", "electronics"})).
    OrderBy("price", qb.Desc).
    Limit(20).
    BuildSelect()

// INSERT with upsert
sql, args, err = qb.New("users").
    OnConflict("(email) DO UPDATE SET name = EXCLUDED.name, updated_at = NOW()").
    BuildInsert(map[string]any{"email": "user@example.com", "name": "Alice"})

// UPDATE
sql, args, err = qb.New("orders").
    Where(qb.Where("id", qb.OpEq, orderID)).
    Returning("id", "updated_at").
    BuildUpdate(map[string]any{"status": "shipped"})

// CTE + subquery
sql, args, err = qb.New("orders").
    With("recent", "SELECT id FROM orders WHERE created_at > $1", cutoff).
    Where(qb.WhereSubquery("id", qb.OpIn, qb.New("recent"))).
    BuildSelect()
```

## Database Client (`db`)

```go
import "github.com/rajangupta9/pgkit/db"

// Typed scanning with generics
type User struct {
    ID    uuid.UUID `db:"id"`
    Name  string    `db:"name"`
    Email string    `db:"email"`
}

users, err := db.QueryInto[User](ctx, client,
    client.QB("users").Where(qb.Where("active", qb.OpEq, true)),
)

// Transactions
err = client.WithTx(ctx, func(tx db.Tx) error {
    id, err := tx.Insert(ctx, tx.QB("orders"), map[string]any{
        "user_id": userID,
        "total":   99.99,
    })
    if err != nil {
        return err
    }
    _, err = tx.Update(ctx, tx.QB("users").Where(qb.Where("id", qb.OpEq, userID)),
        map[string]any{"last_order_id": id},
    )
    return err
})

// SERIALIZABLE retry
err = client.WithRetryTx(ctx, 3, func(tx db.Tx) error {
    // automatically retried on serialization failures (SQLSTATE 40001)
    return nil
})

// Batch dispatch (one round-trip)
b := db.NewBatch()
b.AddSelect(client.QB("users").Where(qb.Where("id", qb.OpEq, uid)))
b.AddExec("UPDATE sessions SET last_seen = NOW() WHERE user_id = $1", uid)
results, err := client.SendWrite(ctx, b)

// LISTEN / NOTIFY
go client.Listen(ctx, "orders", func(n db.Notification) error {
    fmt.Printf("channel=%s payload=%s\n", n.Channel, n.Payload)
    return nil
})
client.Notify(ctx, "orders", `{"id":"abc123"}`)

// Error classification
if db.IsUniqueViolation(err) { /* handle duplicate */ }
if db.IsForeignKeyViolation(err) { /* handle missing reference */ }
if db.IsSerializationFailure(err) { /* safe to retry */ }
```

## Repository Structure

```text
pgkit/
├── doc.go          # root package overview
├── go.mod          # module: github.com/rajangupta9/pgkit
├── LICENSE         # MIT
├── README.md
├── db/             # connection pools, transactions, typed scanning
│   ├── doc.go
│   ├── client.go   # Client, New, Query*, Insert, Update, Delete
│   ├── pool.go     # PoolConfig, NamedPool, poolManager
│   ├── tx.go       # Tx interface, WithTx, WithRetryTx, advisory locks
│   ├── batch.go    # Batch, SendWrite, SendRead
│   ├── scan.go     # QueryInto, InsertInto, UpdateInto, TxQueryInto, …
│   ├── notify.go   # Notify, Listen, ListenMulti
│   ├── errors.go   # error sentinels, Is* predicates, PgError
│   ├── logger.go   # slog-based query logger
│   └── dns.go      # IPv4-preferring DNS resolver
└── qb/             # standalone SQL query builder
    ├── doc.go
    ├── builder.go  # Builder struct, New, Clone, HasReturning
    ├── types.go    # JoinType, SortDir, NullsOrder, LockMode, LockWait
    ├── condition.go# Operator constants, Condition, Where* constructors
    ├── clauses.go  # GroupBy, Having, OrderBy, Limit, Offset, Returning, locking
    ├── select.go   # Columns, Distinct, WindowCol, BuildSelect
    ├── insert.go   # OnConflict, BuildInsert, BuildInsertBatch
    ├── update.go   # BuildUpdate
    ├── delete.go   # BuildDelete
    ├── where.go    # Where, WhereGroup
    ├── join.go     # Join, InnerJoin, LeftJoin, LateralJoin, …
    ├── cte.go      # With, WithRecursive
    ├── union.go    # Union, UnionAll
    ├── quote.go    # QuoteIdent (security-critical)
    └── params.go   # placeholder helpers
```

## Why PGKIT?

- **Separation of concerns** — build SQL (`qb`) independently of executing it (`db`).
- **No magic** — no reflection-based ORM. Every query is explicit and inspectable.
- **Production features** — serialisable retry, advisory locks, batch dispatch, LISTEN/NOTIFY, IPv4 DNS override.
- **Tiny API** — one `Client`, one `Builder`. Everything is chainable and type-safe.
- **pgx foundation** — built on the fastest, most feature-complete Go PostgreSQL driver.

## Documentation

Full API reference on pkg.go.dev:

- [github.com/rajangupta9/pgkit](https://pkg.go.dev/github.com/rajangupta9/pgkit) — root overview
- [github.com/rajangupta9/pgkit/db](https://pkg.go.dev/github.com/rajangupta9/pgkit/db) — database client
- [github.com/rajangupta9/pgkit/qb](https://pkg.go.dev/github.com/rajangupta9/pgkit/qb) — query builder

## Versioning

PGKIT follows [semantic versioning](https://semver.org).

Current stable release: **v1.0.0**

Use tagged releases for reproducible builds:

```bash
go get github.com/rajangupta9/pgkit@v1.0.0
```

## Contributing

1. Fork the repository.
2. Create a feature branch (`git checkout -b feat/my-feature`).
3. Run `go test ./...` and `go vet ./...`.
4. Open a pull request with a clear change summary.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
