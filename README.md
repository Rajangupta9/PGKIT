# PGKIT

Production-grade PostgreSQL toolkit for Go.

PGKIT combines a standalone SQL query builder with a lightweight `pgx`-based database layer for connection pooling, transactions, scan helpers, and PostgreSQL-specific features.

## Features

- `qb`: PostgreSQL-native query builder with safe identifier quoting and parameter binding
- `db`: named pool management, query execution, transaction helpers, and scan helpers
- Support for JSONB, array, full-text search, `ON CONFLICT` upserts, `CTE`s, `UNION`, and row locking
- Typed scanning into Go structs using generics
- `LISTEN` / `NOTIFY` support with dedicated connection handling
- Query timeouts and structured logging via `slog`

## Installation

```bash
go get github.com/rajangupta9/pgkit
```

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
        db.NamedPool{Name: "read", PoolConfig: db.PoolConfig{ConnString: os.Getenv("DATABASE_URL")}},
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    builder := client.QB("users").
        Columns("id", "name", "email").
        Where(qb.Where("active", qb.OpEq, true)).
        OrderBy("created_at", qb.Desc).
        Limit(10)

    rows, err := client.Query(ctx, builder)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(rows)
}
```

## `qb` Query Builder Example

```go
package main

import (
    "fmt"

    "github.com/rajangupta9/pgkit/qb"
)

func main() {
    builder := qb.New("products").
        Columns("id", "name", "price").
        Where(qb.Where("price", qb.OpGt, 100)).
        OrderBy("price", qb.Desc).
        Limit(20)

    sql, args, err := builder.BuildSelect()
    if err != nil {
        panic(err)
    }
    fmt.Println(sql)
    fmt.Println(args)
}
```

## `db` Database Client Example

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
        db.NamedPool{Name: "read", PoolConfig: db.PoolConfig{ConnString: os.Getenv("DATABASE_URL")}},
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    builder := client.QB("orders").
        Columns("id", "total").
        Where(qb.Where("status", qb.OpEq, "paid"))

    rows, err := client.Query(ctx, builder)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(rows)
}
```

## Repository Structure

```text
./
  go.mod          # module definition
  LICENSE         # MIT license
  README.md       # project overview and examples
  db/             # PostgreSQL pool and execution helpers
  qb/             # SQL query builder utilities
```

## Why Use PGKIT

PGKIT is designed for production PostgreSQL workloads with minimal ceremony. It provides a clean separation between SQL construction and execution, while keeping the runtime API small and idiomatic.

## Documentation

See the full API reference on pkg.go.dev:

- https://pkg.go.dev/github.com/rajangupta9/pgkit
- https://pkg.go.dev/github.com/rajangupta9/pgkit/db
- https://pkg.go.dev/github.com/rajangupta9/pgkit/qb

## Versioning

PGKIT follows semantic versioning.

- Stable release: `v1.0.0`
- Use tagged releases for dependable version management

## Contributing

1. Fork the repository.
2. Create a feature branch.
3. Run `go test ./...`.
4. Open a pull request with a clear change summary.

## License

This project is licensed under the MIT License. See the `LICENSE` file for details.
