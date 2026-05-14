# PGKIT
Production-grade PostgreSQL toolkit for Go

## PGKIT — Complete Technical Documentation

> **Version:** Corrected Edition (Post-Audit)  
> **Go Version:** 1.23+  
> **Dependencies:** `github.com/jackc/pgx/v5`, `github.com/google/uuid`  
> **Packages:** `github.com/skolio/pgkit/qb`, `github.com/skolio/pgkit/db`

---

## Table of Contents

1. [Architecture Overview](#1-architecture-overview)
2. [Installation & Quick Start](#2-installation--quick-start)
3. [Package `qb` — Query Builder](#3-package-qb--query-builder)
   - 3.1 [Design Philosophy](#31-design-philosophy)
   - 3.2 [Types & Constants](#32-types--constants)
   - 3.3 [Builder Lifecycle](#33-builder-lifecycle)
   - 3.4 [SELECT Construction](#34-select-construction)
   - 3.5 [WHERE Conditions](#35-where-conditions)
   - 3.6 [JOINs](#36-joins)
   - 3.7 [GROUP BY / HAVING](#37-group-by--having)
   - 3.8 [ORDER BY / LIMIT / OFFSET](#38-order-by--limit--offset)
   - 3.9 [Locking](#39-locking)
   - 3.10 [RETURNING Clauses](#310-returning-clauses)
   - 3.11 [Window Functions](#311-window-functions)
   - 3.12 [CTEs (WITH Clauses)](#312-ctes-with-clauses)
   - 3.13 [UNION / UNION ALL](#313-union--union-all)
   - 3.14 [ON CONFLICT (Upsert)](#314-on-conflict-upsert)
   - 3.15 [INSERT / UPDATE / DELETE Builders](#315-insert--update--delete-builders)
   - 3.16 [Identifier Quoting](#316-identifier-quoting)
   - 3.17 [Error Types](#317-error-types)
   - 3.18 [Parameter Numbering & Nesting](#318-parameter-numbering--nesting)
4. [Package `db` — Connection Pool + Execution](#4-package-db--connection-pool--execution)
   - 4.1 [Client Lifecycle](#41-client-lifecycle)
   - 4.2 [Configuration Reference](#42-configuration-reference)
   - 4.3 [Pool Management](#43-pool-management)
   - 4.4 [Read Operations](#44-read-operations)
   - 4.5 [Write Operations](#45-write-operations)
   - 4.6 [Raw SQL Execution](#46-raw-sql-execution)
   - 4.7 [Typed Scanning (Generics)](#47-typed-scanning-generics)
   - 4.8 [Transactions](#48-transactions)
   - 4.9 [Batch Execution](#49-batch-execution)
   - 4.10 [LISTEN / NOTIFY](#410-listen--notify)
   - 4.11 [Error Helpers](#411-error-helpers)
5. [Security & Safety](#5-security--safety)
6. [Performance & Operational Notes](#6-performance--operational-notes)
7. [Complete API Index](#7-complete-api-index)

---

## 1. Architecture Overview

`pgkit` is split into two independent layers:

| Package | Responsibility | Database Driver? |
|---------|---------------|------------------|
| `qb`    | SQL string + argument construction | No (only uses `pgx` for identifier quoting) |
| `db`    | Connection pools, execution, scanning, transactions | Yes (`pgx/v5`) |

**Dependency rule:** `db` depends on `qb`. `qb` can be used standalone in repository layers that receive a `*pgxpool.Pool` from outside.

---

## 2. Installation & Quick Start

```bash
go get github.com/rajangupta9/pgkit
```

**Minimal example:**

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/skolio/pgkit/db"
    "github.com/skolio/pgkit/qb"
)

func main() {
    ctx := context.Background()

    client, err := db.New(ctx, db.Config{},
        db.NamedPool{
            Name: "write",
            PoolConfig: db.PoolConfig{ConnString: os.Getenv("DATABASE_URL")},
        },
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Insert
    id, err := client.Insert(ctx, client.QB("users"), map[string]any{
        "name":  "Alice",
        "email": "alice@example.com",
    })
    fmt.Println(id, err)
}
```

---

## 3. Package `qb` — Query Builder

### 3.1 Design Philosophy

- **Immutable-ish:** All chain methods return `*Builder`. The builder accumulates state. A `Clone()` method is provided for safe branching.
- **Error Accumulation:** Invalid conditions are stored in an internal error slice. The first error is returned on the first `Build*` call.
- **PostgreSQL-native:** Uses `$1, $2…` placeholders, double-quoted identifiers, and supports PG-specific operators (`@>`, `?&`, `@@`, etc.).
- **No driver dependency:** Only imports `pgx` for `pgx.Identifier.Sanitize()`.

### 3.2 Types & Constants

#### JoinType
```go
type JoinType string
const (
    JoinInner JoinType = "INNER"
    JoinLeft  JoinType = "LEFT"
    JoinRight JoinType = "RIGHT"
    JoinFull  JoinType = "FULL"
    JoinCross JoinType = "CROSS"
)
```

#### SortDir
```go
type SortDir string
const (
    Asc  SortDir = "ASC"
    Desc SortDir = "DESC"
)
```

#### NullsOrder
```go
type NullsOrder string
const (
    NullsFirst NullsOrder = "NULLS FIRST"
    NullsLast  NullsOrder = "NULLS LAST"
)
```

#### LockMode
```go
type LockMode string
const (
    LockForUpdate      LockMode = "FOR UPDATE"
    LockForShare       LockMode = "FOR SHARE"
    LockForNoKeyUpdate LockMode = "FOR NO KEY UPDATE"
    LockForKeyShare    LockMode = "FOR KEY SHARE"
)
```

#### LockWait
```go
type LockWait string
const (
    Wait       LockWait = ""           // block until lock available
    NoWait     LockWait = "NOWAIT"      // error immediately if locked
    SkipLocked LockWait = "SKIP LOCKED" // skip locked rows
)
```

#### Operator
```go
type Operator string
```

**Scalar comparison:**
- `OpEq` (`=`), `OpNotEq` (`<>`), `OpLt` (`<`), `OpLte` (`<=`), `OpGt` (`>`), `OpGte` (`>=`)

**Pattern matching:**
- `OpLike` (`LIKE`), `OpILike` (`ILIKE`)

**Membership:**
- `OpIn` (`IN`), `OpNotIn` (`NOT IN`)

**Null checks (valueless):**
- `OpIsNull` (`IS NULL`), `OpNotNull` (`IS NOT NULL`)

**Range:**
- `OpBetween` (`BETWEEN`), `OpNotBetween` (`NOT BETWEEN`)

**Array operators (value must be a Go slice bound as a pgx array):**
- `OpAny` (`= ANY`), `OpAll` (`= ALL`)

**Array containment:**
- `OpArrayContains` (`@>`), `OpArrayContainedBy` (`<@`), `OpArrayOverlap` (`&&`)

**JSONB operators:**
- `OpJSONContains` (`@>`), `OpJSONContainedBy` (`<@`), `OpJSONHasKey` (`?`), `OpJSONHasAllKeys` (`?&`), `OpJSONHasAnyKey` (`?|`)

**Full-text search:**
- `OpTextSearch` (`@@`)

**Internal operators (used by condition constructors):**
- `OpExists`, `OpNotExists`, `OpSubquery`, `OpRaw`

#### Condition
```go
type Condition struct {
    Column   string
    Operator Operator
    Value    any
    Sub      *Builder   // set for EXISTS / subquery conditions
}
```

**Validation rules:**
- `Column` must be non-empty for all operators except `OpExists`, `OpNotExists`, `OpRaw`.
- `Sub` must be non-nil for `OpExists`, `OpNotExists`, `OpSubquery`.

### 3.3 Builder Lifecycle

#### `qb.New(table string) *Builder`
Creates a new builder targeting `table`. `table` can be:
- Plain: `"users"`
- Schema-qualified: `"public.users"`
- Aliased: `"users u"` or `"public.users u"`

#### `(*Builder) Clone() *Builder`
Returns a **deep copy** of the builder. Safe to modify without affecting the original. Returns `nil` if called on a nil receiver.

#### `(*Builder) HasReturning() bool`
Reports whether the builder has an explicit RETURNING clause (any mode except `returningUnset` and `returningNone`). Used internally by the batch API to decide whether a statement returns rows.

### 3.4 SELECT Construction

#### `(*Builder) Columns(cols ...string) *Builder`
Appends expressions to the SELECT list.

**Quoting rules:**
- Plain names → `"name"`
- Qualified names → `"alias"."col"`
- Expressions containing `(`, `)`, `*`, `/`, or space → written verbatim

```go
qb.New("users").Columns("id", "name")
// → SELECT "id", "name" …

qb.New("orders").Columns("COUNT(*) AS total", "u.email")
// → SELECT COUNT(*) AS total, "u"."email" …
```

#### `(*Builder) Distinct() *Builder`
Adds `DISTINCT` to the SELECT clause.

### 3.5 WHERE Conditions

#### `(*Builder) Where(c Condition) *Builder`
Appends a single condition ANDed into WHERE. Validates the condition immediately; if invalid, stores an error in the builder.

#### `(*Builder) WhereGroup(g condGroup) *Builder`
Appends a group of conditions ANDed into WHERE. If the group is an OR-group, every condition inside is validated.

#### Condition Constructors

| Function | Signature | SQL Output |
|----------|-----------|------------|
| `Where` | `(col string, op Operator, val any)` | `"col" = $1` |
| `WhereNull` | `(col string)` | `"col" IS NULL` |
| `WhereNotNull` | `(col string)` | `"col" IS NOT NULL` |
| `WhereIn` | `(col string, vals any)` | `"col" IN ($1, $2)` |
| `WhereNotIn` | `(col string, vals any)` | `"col" NOT IN ($1, $2)` |
| `WhereBetween` | `(col string, low, high any)` | `"col" BETWEEN $1 AND $2` |
| `WhereNotBetween` | `(col string, low, high any)` | `"col" NOT BETWEEN $1 AND $2` |
| `WhereAny` | `(col string, val any)` | `"col" = ANY($1)` |
| `WhereAll` | `(col string, val any)` | `"col" = ALL($1)` |
| `WhereJSONContains` | `(col string, val any)` | `"col" @> $1` |
| `WhereJSONHasKey` | `(col, key string)` | `"col" ? $1` |
| `WhereArrayContains` | `(col string, val any)` | `"col" @> $1` |
| `WhereArrayOverlap` | `(col string, val any)` | `"col" && $1` |
| `WhereTextSearch` | `(col, tsquery string)` | `"col" @@ $1` |
| `WhereExists` | `(sub *Builder)` | `EXISTS (SELECT …)` |
| `WhereNotExists` | `(sub *Builder)` | `NOT EXISTS (SELECT …)` |
| `WhereSubquery` | `(col string, op Operator, sub *Builder)` | `"col" IN (SELECT …)` |
| `WhereRaw` | `(expr string, args ...any)` | verbatim expression, `?` → `$N` |
| `OrGroup` | `(conds ...Condition)` | `(a OR b OR c)` |

**`WhereRaw` placeholder rules:**
- Use `?` as placeholder in the expression string.
- Each `?` is replaced with `$N` in order.
- Arguments are bound positionally.

```go
qb.WhereRaw("lower(email) = ? AND created_at > ?", "foo@bar.com", someTime)
// → lower(email) = $1 AND created_at > $2
```

**`WhereIn` / `WhereNotIn` validation:**
- Value must be a slice or `[]any`.
- Empty slices are rejected at build time.

### 3.6 JOINs

#### `(*Builder) Join(kind JoinType, table, condition string) *Builder`
Appends a JOIN. `table` and `condition` are written **verbatim** — never pass user input.

Sugar methods:
- `InnerJoin(table, condition)`
- `LeftJoin(table, condition)`
- `RightJoin(table, condition)`
- `FullJoin(table, condition)`

**CROSS JOIN:** Pass `JoinCross` with `""` as condition.

#### `(*Builder) LateralJoin(kind JoinType, sub *Builder, alias string) *Builder`
Appends a `LATERAL` subquery join. The subquery builder's parameters are automatically continued from the outer query's current index.

```go
sub := qb.New("order_items").Columns("product_id", "qty").Where(qb.Where("order_id", qb.OpEq, 5)).Limit(3)
qb.New("orders").Columns("orders.id", "items.product_id").
    LateralJoin(qb.JoinLeft, sub, "items")
// → … LEFT JOIN LATERAL (SELECT "product_id", "qty" FROM "order_items" WHERE "order_id" = $1 LIMIT 3) "items" ON TRUE
```

**Safety:** Passing a nil subquery stores an error in the builder rather than panicking.

### 3.7 GROUP BY / HAVING

#### `(*Builder) GroupBy(cols ...string) *Builder`
Appends columns to `GROUP BY`. Columns are auto-quoted.

#### `(*Builder) Having(expr string, args ...any) *Builder`
Sets a raw HAVING expression. Uses `?` placeholders rewritten to `$N`.

**Parameter continuation:** HAVING arguments continue numbering after WHERE arguments.

```go
qb.New("orders").
    Where(qb.Where("school_id", qb.OpEq, 3)).
    GroupBy("user_id").
    Having("SUM(total) > ?", 500)
// WHERE "school_id" = $1 GROUP BY "user_id" HAVING SUM(total) > $2
```

### 3.8 ORDER BY / LIMIT / OFFSET

#### `(*Builder) OrderBy(col string, dir SortDir, nulls ...NullsOrder) *Builder`
Appends an ORDER BY clause. `nulls` is optional.

```go
qb.New("users").OrderBy("score", qb.Desc, qb.NullsLast)
// → ORDER BY "score" DESC NULLS LAST
```

#### `(*Builder) Limit(n int) *Builder`
Sets `LIMIT n`. Written as an integer literal (safe from injection because `n` is an `int`). `0` = no limit.

#### `(*Builder) Offset(n int) *Builder`
Sets `OFFSET n`. Written as an integer literal. `0` = no offset.

### 3.9 Locking

#### `(*Builder) ForUpdate(wait LockWait) *Builder`
Appends `FOR UPDATE [NOWAIT | SKIP LOCKED]`.

#### `(*Builder) ForShare(wait LockWait) *Builder`
Appends `FOR SHARE [NOWAIT | SKIP LOCKED]`.

#### `(*Builder) Lock(mode LockMode, wait LockWait) *Builder`
Sets a custom lock mode.

### 3.10 RETURNING Clauses

RETURNING behavior depends on the build method and the builder's explicit setting:

| Method | Default if unset | Override methods |
|--------|------------------|------------------|
| `BuildInsert` | `RETURNING "id"` | `ReturningAll()`, `Returning(cols...)`, `ReturningNone()` |
| `BuildUpdate` | no RETURNING | `ReturningAll()`, `Returning(cols...)`, `ReturningNone()` |
| `BuildDelete` | no RETURNING | `ReturningAll()`, `Returning(cols...)`, `ReturningNone()` |

#### Methods
- `ReturningID()` — `RETURNING "id"`
- `ReturningAll()` — `RETURNING *`
- `Returning(cols ...string)` — `RETURNING "a", "b"…`
- `ReturningNone()` — suppress RETURNING entirely

### 3.11 Window Functions

#### `(*Builder) WindowCol(fn, partitionBy, orderBy, alias string) *Builder`
Appends a window function expression to the SELECT list.

| Argument | Purpose | Empty string behavior |
|----------|---------|----------------------|
| `fn` | Window function call | Required (e.g. `"RANK()"`) |
| `partitionBy` | PARTITION BY column | Omitted |
| `orderBy` | ORDER BY expression | Omitted |
| `alias` | AS name | Auto-quoted |

```go
qb.New("orders").
    Columns("user_id", "total").
    WindowCol("RANK()", "user_id", "total DESC", "rank")
// → SELECT "user_id", "total", RANK() OVER (PARTITION BY "user_id" ORDER BY total DESC) AS "rank" …
```

### 3.12 CTEs (WITH Clauses)

#### `(*Builder) With(name, query string, args ...any) *Builder`
Prepends a non-recursive CTE.

#### `(*Builder) WithRecursive(name, query string, args ...any) *Builder`
Prepends a recursive CTE.

**Parameter numbering correction:** CTE query strings may contain `$N` placeholders. The builder **automatically renumbers** all `$N` inside CTE queries so they align with the global argument list. CTE arguments are prepended first.

```go
qb.New("recent").
    With("recent", "SELECT id, total FROM orders WHERE created_at > $1", cutoff).
    Columns("id", "total").
    Where(qb.Where("total", qb.OpGt, 100))
// WITH "recent" AS (SELECT id, total FROM orders WHERE created_at > $1)
//   SELECT "id", "total" FROM "recent" WHERE "total" > $2
// Args: [cutoff, 100]
```

**Multi-CTE safety:** If multiple CTEs use `$1` in their raw strings, the builder offsets them so each CTE receives its own arguments.

### 3.13 UNION / UNION ALL

#### `(*Builder) Union(other *Builder) *Builder`
Appends `UNION` (deduplicating). Nil builders are rejected gracefully (error stored).

#### `(*Builder) UnionAll(other *Builder) *Builder`
Appends `UNION ALL`. Nil builders rejected gracefully.

**Parameter continuation:** The right-hand builder's parameters continue from the left-hand builder's last index.

```go
a := qb.New("students").Where(qb.Where("school_id", qb.OpEq, 1))
b := qb.New("alumni").Where(qb.Where("school_id", qb.OpEq, 2))
a.UnionAll(b).BuildSelect()
// → SELECT * FROM "students" WHERE "school_id" = $1
//   UNION ALL SELECT * FROM "alumni" WHERE "school_id" = $2
```

### 3.14 ON CONFLICT (Upsert)

#### `(*Builder) OnConflict(clause string) *Builder`
Sets raw ON CONFLICT clause for INSERT. Written verbatim.

```go
qb.New("users").
    OnConflict(`(email) DO UPDATE SET name = EXCLUDED.name, updated_at = NOW()`).
    BuildInsert(data)
```

### 3.15 INSERT / UPDATE / DELETE Builders

#### `(*Builder) BuildSelect() (sql string, args []any, err error)`
Produces a full parameterized SELECT starting at `$1`.

#### `(*Builder) BuildInsert(data map[string]any) (sql string, args []any, err error)`
Produces `INSERT INTO … (cols) VALUES ($1,…) [ON CONFLICT …] [RETURNING …]`.

**Validation:**
- `data` must not be empty.
- Columns are sorted alphabetically for deterministic output.
- Default RETURNING is `"id"`.

#### `(*Builder) BuildInsertBatch(rows []map[string]any) (sql string, args []any, err error)`
Produces multi-row INSERT.

**Validation:**
- `rows` must not be empty.
- Row 0 defines the column list; all subsequent rows must contain the same keys.
- No row may be an empty map.
- Default RETURNING is `"id"`.

#### `(*Builder) BuildUpdate(data map[string]any) (sql string, args []any, err error)`
Produces `UPDATE … SET col=$1,… WHERE … [RETURNING …]`.

**Validation:**
- `data` must not be empty.
- SET clauses are sorted alphabetically.
- WHERE parameters continue after SET parameters.

#### `(*Builder) BuildDelete() (sql string, args []any, err error)`
Produces `DELETE FROM … WHERE … [RETURNING …]`.

**Global limit:** All `Build*` methods reject queries with more than **65,535** arguments (PostgreSQL protocol limit).

### 3.16 Identifier Quoting

#### `qb.QuoteIdent(name string) string`
Safely quotes a single PostgreSQL identifier using `pgx.Identifier.Sanitize()`.

```go
qb.QuoteIdent("my table") // → `"my table"`
```

**Internal rules (`quoteIdentExpr`):**
1. If the expression contains `(`, `)`, `*`, `/`, or space → written verbatim (assumed raw SQL).
2. If it contains `.` → split and quote each part: `alias.col` → `"alias"."col"`.
3. If already wrapped in double quotes → returned as-is.
4. Otherwise → quoted via `pgx.Identifier.Sanitize()`.

**Table expressions (`quoteTableExpr`):**
- Handles plain names, `schema.table`, and `table alias`.
- Aliases are auto-quoted.

### 3.17 Error Types

```go
var (
    ErrNoTable   = fmt.Errorf("qb: table name must not be empty")
    ErrEmptyData = fmt.Errorf("qb: data map must not be empty")
    ErrEmptyRows = fmt.Errorf("qb: batch rows must not be empty")
)
```

**Error accumulation:** The builder stores the first error encountered during chaining. Calling any `Build*` returns that stored error immediately.

### 3.18 Parameter Numbering & Nesting

The builder maintains a monotonically increasing parameter index across:
- CTE arguments (prepended first, then renumbered)
- Main WHERE conditions
- JOIN lateral subqueries
- HAVING expressions
- UNION / UNION ALL right-hand sides
- EXISTS / NOT EXISTS subqueries
- Subquery conditions (`col IN (SELECT …)`)

**Example — nested subqueries:**
```go
outer := qb.New("users").Where(qb.Where("active", qb.OpEq, true))
mid := qb.New("orders").Where(qb.Where("user_id", qb.OpEq, 99))
inner := qb.New("items").Where(qb.Where("order_id", qb.OpEq, 42))

outer.Where(qb.WhereExists(
    mid.Where(qb.WhereExists(inner)),
)).BuildSelect()
// $1 = true (outer)
// $2 = 99   (mid)
// $3 = 42   (inner)
```

---

## 4. Package `db` — Connection Pool + Execution

### 4.1 Client Lifecycle

#### `db.New(ctx context.Context, cfg Config, pools ...NamedPool) (*Client, error)`
Creates a client and establishes all named pools concurrently.

**Requirements:**
- At least one `NamedPool` is required.
- Pool names must be non-empty and unique.
- Each pool's `ConnString` must be non-empty.
- A startup ping is performed on every pool using `ConnectTimeout`.
- If any pool fails, all successfully created pools are closed and an error is returned.

#### `(*Client) Close()`
Closes all registered pools. Call during graceful shutdown.

#### `(*Client) HealthCheck(ctx context.Context) error`
Pings all pools concurrently. Returns a joined error if any pool fails.

### 4.2 Configuration Reference

#### `Config`
```go
type Config struct {
    QueryTimeout time.Duration   // Per-query context timeout. 0 = no limit.
    Logger       *slog.Logger    // Defaults to slog.Default() if nil.
}
```

#### `PoolConfig`
```go
type PoolConfig struct {
    ConnString        string        // Required. Full PostgreSQL DSN.
    MaxConns          int32         // Default: 10
    MinConns          int32         // Default: 2
    MaxConnIdleTime   time.Duration // Default: 5m
    MaxConnLifetime   time.Duration // Default: 1h
    HealthCheckPeriod time.Duration // Default: 1m
    ConnectTimeout    time.Duration // Default: 20s
    ForceIPv4         bool          // Default: false
}
```

#### `NamedPool`
```go
type NamedPool struct {
    Name string
    PoolConfig
}
```

**DNS / IPv4 behavior:**
- `ForceIPv4 = false` (default): Returns IPv4 addresses first, IPv6 as fallback. Required for GCP Cloud Run where IPv6 times out.
- `ForceIPv4 = true`: Fails hard if no A record exists. Use on VMs without IPv6 routes.

**QueryExecMode:** The client configures `pgx.QueryExecModeSimpleProtocol` on every pool. This disables prepared-statement caching and binary encoding, ensuring compatibility with PgBouncer in transaction mode. Slightly slower than prepared protocol but avoids prepared-statement cache synchronization issues.

### 4.3 Pool Management

#### `(*Client) Pool(name string) *pgxpool.Pool`
Returns the raw pool by name, or `nil` if not registered.

#### `(*Client) QB(table string) *qb.Builder`
Convenience factory. Equivalent to `qb.New(table)`.

### 4.4 Read Operations

All read operations target the `"read"` pool by convention, unless noted.

#### `(*Client) Query(ctx context.Context, b *qb.Builder) ([]map[string]any, error)`
Executes `b.BuildSelect()` on the `"read"` pool. Returns all rows as `[]map[string]any`.

#### `(*Client) QueryOne(ctx context.Context, b *qb.Builder) (map[string]any, error)`
Executes `b.BuildSelect()` with an internal `LIMIT 1` on the `"read"` pool. Returns `ErrNoRows` if no match.

**Important:** This method **clones** the builder before injecting `Limit(1)`. The caller's builder is never mutated.

#### `(*Client) QueryWrite(ctx context.Context, b *qb.Builder) ([]map[string]any, error)`
Executes a SELECT on the `"write"` pool. Use immediately after a write to avoid read-replica lag.

#### `(*Client) QueryPool(ctx context.Context, poolName string, b *qb.Builder) ([]map[string]any, error)`
Executes a SELECT on the named pool.

#### `(*Client) QuerySQL(ctx context.Context, sql string, args ...any) ([]map[string]any, error)`
Executes raw SQL SELECT on the `"read"` pool. Signature is variadic for consistency with `ExecSQL`.

### 4.5 Write Operations

All write operations target the `"write"` pool by convention.

#### `(*Client) Insert(ctx context.Context, b *qb.Builder, data map[string]any) (uuid.UUID, error)`
Executes `b.BuildInsert(data)` on the `"write"` pool. Scans and returns the generated UUID from `RETURNING "id"`.

#### `(*Client) InsertBatch(ctx context.Context, b *qb.Builder, rows []map[string]any) (uuid.UUID, error)`
Executes `b.BuildInsertBatch(rows)` on the `"write"` pool. Returns the **first** generated UUID from `RETURNING "id"` (not the last).

#### `(*Client) Update(ctx context.Context, b *qb.Builder, data map[string]any) (int64, error)`
Executes `b.BuildUpdate(data)` on the `"write"` pool. Returns rows affected.

#### `(*Client) Delete(ctx context.Context, b *qb.Builder) (int64, error)`
Executes `b.BuildDelete()` on the `"write"` pool. Returns rows affected.

#### `(*Client) ExecSQL(ctx context.Context, sql string, args ...any) (int64, error)`
Executes raw write SQL on the `"write"` pool. Returns rows affected.

#### `(*Client) ExecPoolSQL(ctx context.Context, poolName string, sql string, args ...any) (int64, error)`
Executes raw write SQL on the named pool.

### 4.6 Typed Scanning (Generics)

All generic functions use `pgx.RowToStructByName[T]`, which requires struct fields to be tagged with `db:"column_name"`.

#### `db.QueryInto[T any](ctx context.Context, c *Client, b *qb.Builder) ([]T, error)`
Executes SELECT and scans all rows into `[]T`.

```go
type User struct {
    ID   uuid.UUID `db:"id"`
    Name string    `db:"name"`
}
users, err := db.QueryInto[User](ctx, client, client.QB("users").Limit(20))
```

#### `db.QueryOneInto[T any](ctx context.Context, c *Client, b *qb.Builder) (*T, error)`
Executes SELECT with LIMIT 1 and scans into `*T`. Returns `ErrNoRows` if empty.

**Important:** Clones the builder before injecting `Limit(1)`.

#### `db.InsertInto[T any](ctx context.Context, c *Client, b *qb.Builder, data map[string]any) (*T, error)`
Executes INSERT and scans RETURNING columns into `*T`. The builder must have `Returning(…)` or `ReturningAll()` set.

#### `db.UpdateInto[T any](ctx context.Context, c *Client, b *qb.Builder, data map[string]any) (*T, error)`
Executes UPDATE and scans the first RETURNING row into `*T`. Requires RETURNING on the builder.

#### `db.TxQueryInto[T any](ctx context.Context, tx pgx.Tx, b *qb.Builder) ([]T, error)`
Executes SELECT inside a raw `pgx.Tx` and scans rows into `[]T`.

#### `db.TxQueryOneInto[T any](ctx context.Context, tx pgx.Tx, b *qb.Builder) (*T, error)`
Executes SELECT with LIMIT 1 inside a raw `pgx.Tx`.

**Important:** Clones the builder before injecting `Limit(1)`.

#### `db.TxInsertInto[T any](ctx context.Context, tx pgx.Tx, b *qb.Builder, data map[string]any) (*T, error)`
Executes INSERT inside a raw `pgx.Tx` and scans RETURNING into `*T`.

#### `db.TxUpdateInto[T any](ctx context.Context, tx pgx.Tx, b *qb.Builder, data map[string]any) (*T, error)`
Executes UPDATE inside a raw `pgx.Tx` and scans RETURNING into `*T`.

#### `db.ScanUUID(row pgx.Row) (uuid.UUID, error)`
Scans a single UUID from a `QueryRow` result. Returns `ErrNoRows` if applicable.

### 4.7 Transactions

#### `(*Client) WithTx(ctx context.Context, fn func(Tx) error) error`
Runs `fn` inside a transaction on the `"write"` pool. Commits on nil return; rolls back on error.

**Panic safety:** If `fn` panics, the transaction is rolled back before the panic is re-raised. This prevents connection leaks.

#### `(*Client) WithTxOpts(ctx context.Context, opts TxOptions, fn func(Tx) error) error`
Like `WithTx` but with custom isolation level and access mode.

```go
client.WithTxOpts(ctx, db.TxOptions{IsoLevel: pgx.RepeatableRead}, func(tx db.Tx) error {
    // …
})
```

#### `(*Client) WithPoolTx(ctx context.Context, poolName string, fn func(Tx) error) error`
Runs `fn` in a transaction on the named pool.

#### `(*Client) WithRetryTx(ctx context.Context, maxRetries int, fn func(Tx) error) error`
Runs `fn` in a `SERIALIZABLE` transaction on the `"write"` pool and retries automatically on serialization failures (`40001`).

**Retry behavior:**
- Includes **exponential backoff** between retries (10ms, 20ms, 40ms…).
- `maxRetries` is the total attempt count (not retries-after-first).
- Returns an error wrapping the last serialization failure if all attempts exhaust.

#### `Tx` Interface
```go
type Tx interface {
    QB(table string) *qb.Builder

    Insert(ctx context.Context, b *qb.Builder, data map[string]any) (uuid.UUID, error)
    Update(ctx context.Context, b *qb.Builder, data map[string]any) (int64, error)
    Delete(ctx context.Context, b *qb.Builder) (int64, error)
    Select(ctx context.Context, b *qb.Builder) ([]map[string]any, error)
    SelectOne(ctx context.Context, b *qb.Builder) (map[string]any, error)

    ExecRaw(ctx context.Context, sql string, args ...any) (int64, error)
    QueryRaw(ctx context.Context, sql string, args ...any) ([]map[string]any, error)

    Savepoint(ctx context.Context, name string) error
    RollbackTo(ctx context.Context, name string) error
    ReleaseSavepoint(ctx context.Context, name string) error
}
```

**`SelectOne` safety:** Clones the builder before injecting `Limit(1)`.

#### Savepoints
```go
client.WithTx(ctx, func(tx db.Tx) error {
    tx.Savepoint(ctx, "before_charge")
    if err := charge(ctx, tx); err != nil {
        tx.RollbackTo(ctx, "before_charge")
    }
    tx.ReleaseSavepoint(ctx, "before_charge")
    return nil
})
```

Savepoint names are safely quoted via `qb.QuoteIdent`.

#### Advisory Locks
```go
// Blocking
err := db.AcquireAdvisoryLock(ctx, tx, userID)

// Non-blocking
ok, err := db.TryAdvisoryLock(ctx, tx, userID)
```

Both use transaction-scoped locks (`pg_advisory_xact_lock*`), released automatically when the transaction ends.

### 4.8 Batch Execution

#### `db.NewBatch() *Batch`
Creates an empty batch.

#### Batch Building Methods

| Method | Returns rows? | Description |
|--------|---------------|-------------|
| `Add(sql, args...)` | Yes | Raw SQL that returns rows |
| `AddExec(sql, args...)` | No | Raw SQL that does not return rows |
| `AddSelect(builder)` | Yes | SELECT from builder |
| `AddInsert(builder, data)` | Yes | INSERT (always returns rows due to default RETURNING) |
| `AddUpdate(builder, data)` | Conditional | Returns rows only if builder has explicit RETURNING |
| `AddDelete(builder)` | Conditional | Returns rows only if builder has explicit RETURNING |

**Important correction:** The batch API now correctly distinguishes between statements that return rows (`br.Query()`) and statements that do not (`br.Exec()`). If you use `AddDelete` on a builder without RETURNING, the batch executes it via `Exec()` and places `nil` in the results slice for that slot.

**Error accumulation:** If any builder method fails during batch construction, the first error is stored. `SendWrite` / `SendRead` return that error immediately without sending anything to the database.

#### `(*Client) SendWrite(ctx context.Context, b *Batch) ([][]map[string]any, error)`
Sends the batch over the `"write"` pool.

#### `(*Client) SendRead(ctx context.Context, b *Batch) ([][]map[string]any, error)`
Sends the batch over the `"read"` pool.

**Result shape:** `results[i]` corresponds to `b.entries[i]`. For non-returning statements, `results[i]` is `nil`.

### 4.9 LISTEN / NOTIFY

#### `(*Client) Notify(ctx context.Context, channel, payload string) error`
Sends a notification. Channel name is safely quoted; payload is a bound parameter.

#### `(*Client) Listen(ctx context.Context, channel string, handler func(Notification) error) error`
Acquires a dedicated connection from the `"write"` pool, issues `LISTEN channel`, and blocks calling `handler` for every notification until `ctx` is cancelled.

**Operational warning:** This holds a connection open indefinitely. Ensure your pool `MaxConns` accounts for long-running listeners. If the handler returns an error, listening stops and the error is returned.

#### `(*Client) ListenMulti(ctx context.Context, channels []string, handler func(Notification) error) error`
Listens on multiple channels simultaneously on a single dedicated connection.

```go
type Notification struct {
    Channel string
    Payload string
    PID     uint32
}
```

### 4.10 Error Helpers

```go
var (
    ErrNoRows    = errors.New("db: no rows found")
    ErrEmptyRows = errors.New("db: batch rows must not be empty")
)
```

#### Predicate Functions

| Function | PG Code | Description |
|----------|---------|-------------|
| `IsNoRows(err)` | — | `true` if `ErrNoRows` or `pgx.ErrNoRows` |
| `IsUniqueViolation(err)` | `23505` | Unique constraint violation |
| `IsForeignKeyViolation(err)` | `23503` | Foreign key violation |
| `IsNotNullViolation(err)` | `23502` | NOT NULL violation |
| `IsCheckViolation(err)` | `23514` | CHECK constraint violation |
| `IsDeadlock(err)` | `40P01` | Deadlock detected |
| `IsSerializationFailure(err)` | `40001` | Serializable conflict (safe to retry) |
| `IsInvalidTextRepresentation(err)` | `22P02` | Invalid input syntax (malformed UUID, enum, etc.) |
| `IsUndefinedTable(err)` | `42P01` | Missing table |
| `IsConnectionException(err)` | `08xxx` | Connection failure |

#### `db.PgError(err error) (*pgconn.PgError, bool)`
Unwraps the raw PostgreSQL error. Returns the concrete `*pgconn.PgError` and `true` if present; otherwise `nil, false`.

**Fields available on `*pgconn.PgError`:**
- `Code` — SQLSTATE
- `Message` — human-readable message
- `Detail`, `Hint` — additional context
- `Schema`, `Table`, `Column`, `ConstraintName` — affected objects

---

## 5. Security & Safety

### SQL Injection Risks (By Design)

Several APIs accept raw SQL fragments by design. **Never pass user input to these methods.**

| API | Accepted Raw Input |
|-----|-------------------|
| `Columns("expr")` | Raw expressions with `(` / `)` / space |
| `Join(..., condition)` | ON condition |
| `Having(expr, ...)` | HAVING expression |
| `WindowCol(fn, ..., orderBy, ...)` | `fn` and `orderBy` |
| `OnConflict(clause)` | Full ON CONFLICT clause |
| `WhereRaw(expr, ...)` | Raw WHERE fragment |
| `With(name, query, ...)` | CTE query string |

**Mitigation:**
- Use bound parameters (`?` or builder conditions) for all user data.
- Validate/sanitize table/column names via an allowlist before passing to `qb.New` or `Columns`.

### Builder Mutation Safety

The corrected codebase uses `Clone()` internally in all "One" methods (`QueryOne`, `QueryOneInto`, `SelectOne`, `TxQueryOneInto`). If you write helper functions that mutate builders, clone them explicitly:

```go
func MyHelper(b *qb.Builder) *qb.Builder {
    b2 := b.Clone()
    b2.Where(qb.Where("active", qb.OpEq, true))
    return b2
}
```

### Pool Exhaustion

- **Listeners** (`Listen`, `ListenMulti`) hold dedicated connections forever.
- **Transactions** hold a connection for their duration.
- Size pools accordingly: `MaxConns >= (app_concurrency + listeners + batch_concurrency)`.

---

## 6. Performance & Operational Notes

### Simple Protocol Mode
All pools use `pgx.QueryExecModeSimpleProtocol`. This means:
- No prepared statement cache per connection.
- Parameters are sent as text.
- Compatible with PgBouncer transaction mode.
- Slightly higher CPU usage for large binary types (UUID, bytea, arrays) due to text encoding.

### Query Timeout
Set `Config.QueryTimeout` to enforce a hard ceiling on all queries executed through the high-level API. Raw `pgxpool.Pool` usage bypasses this.

### Logging
The executor logs every query at `Debug` level (success) or `Error` level (failure). Attributes include:
- `op` — operation name (`select`, `insert`, `tx-write`, etc.)
- `dur` — duration
- `err` — error string (if any)

### IPv4 Preference
If you see connection timeouts on GCP Cloud Run or similar environments, set `ForceIPv4: true` in `PoolConfig`.

---

## 7. Complete API Index

### `qb` Package

**Types:** `Builder`, `Condition`, `JoinType`, `SortDir`, `NullsOrder`, `LockMode`, `LockWait`, `Operator`, `condGroup`

**Constants:** `JoinInner`, `JoinLeft`, `JoinRight`, `JoinFull`, `JoinCross`, `Asc`, `Desc`, `NullsFirst`, `NullsLast`, `LockForUpdate`, `LockForShare`, `LockForNoKeyUpdate`, `LockForKeyShare`, `Wait`, `NoWait`, `SkipLocked`, `OpEq`, `OpNotEq`, `OpLt`, `OpLte`, `OpGt`, `OpGte`, `OpLike`, `OpILike`, `OpIn`, `OpNotIn`, `OpIsNull`, `OpNotNull`, `OpBetween`, `OpNotBetween`, `OpAny`, `OpAll`, `OpArrayContains`, `OpArrayContainedBy`, `OpArrayOverlap`, `OpJSONContains`, `OpJSONContainedBy`, `OpJSONHasKey`, `OpJSONHasAllKeys`, `OpJSONHasAnyKey`, `OpTextSearch`, `OpExists`, `OpNotExists`, `OpSubquery`, `OpRaw`, `MaxQueryParams`

**Functions:**
- `New(table string) *Builder`
- `Where(col string, op Operator, val any) Condition`
- `WhereNull(col string) Condition`
- `WhereNotNull(col string) Condition`
- `WhereIn(col string, vals any) Condition`
- `WhereNotIn(col string, vals any) Condition`
- `WhereBetween(col string, low, high any) Condition`
- `WhereNotBetween(col string, low, high any) Condition`
- `WhereAny(col string, val any) Condition`
- `WhereAll(col string, val any) Condition`
- `WhereJSONContains(col string, val any) Condition`
- `WhereJSONHasKey(col, key string) Condition`
- `WhereArrayContains(col string, val any) Condition`
- `WhereArrayOverlap(col string, val any) Condition`
- `WhereTextSearch(col, tsquery string) Condition`
- `WhereExists(sub *Builder) Condition`
- `WhereNotExists(sub *Builder) Condition`
- `WhereSubquery(col string, op Operator, sub *Builder) Condition`
- `WhereRaw(expr string, args ...any) Condition`
- `OrGroup(conds ...Condition) condGroup`
- `QuoteIdent(name string) string`

**Builder Methods:**
- `Clone() *Builder`
- `HasReturning() bool`
- `Columns(cols ...string) *Builder`
- `Distinct() *Builder`
- `Join(kind JoinType, table, condition string) *Builder`
- `InnerJoin(table, condition string) *Builder`
- `LeftJoin(table, condition string) *Builder`
- `RightJoin(table, condition string) *Builder`
- `FullJoin(table, condition string) *Builder`
- `LateralJoin(kind JoinType, sub *Builder, alias string) *Builder`
- `Where(c Condition) *Builder`
- `WhereGroup(g condGroup) *Builder`
- `GroupBy(cols ...string) *Builder`
- `Having(expr string, args ...any) *Builder`
- `OrderBy(col string, dir SortDir, nulls ...NullsOrder) *Builder`
- `Limit(n int) *Builder`
- `Offset(n int) *Builder`
- `ForUpdate(wait LockWait) *Builder`
- `ForShare(wait LockWait) *Builder`
- `Lock(mode LockMode, wait LockWait) *Builder`
- `ReturningID() *Builder`
- `ReturningAll() *Builder`
- `Returning(cols ...string) *Builder`
- `ReturningNone() *Builder`
- `WindowCol(fn, partitionBy, orderBy, alias string) *Builder`
- `With(name, query string, args ...any) *Builder`
- `WithRecursive(name, query string, args ...any) *Builder`
- `Union(other *Builder) *Builder`
- `UnionAll(other *Builder) *Builder`
- `OnConflict(clause string) *Builder`
- `BuildSelect() (sql string, args []any, err error)`
- `BuildInsert(data map[string]any) (sql string, args []any, err error)`
- `BuildInsertBatch(rows []map[string]any) (sql string, args []any, err error)`
- `BuildUpdate(data map[string]any) (sql string, args []any, err error)`
- `BuildDelete() (sql string, args []any, err error)`

**Errors:** `ErrNoTable`, `ErrEmptyData`, `ErrEmptyRows`

### `db` Package

**Types:** `Client`, `Config`, `PoolConfig`, `NamedPool`, `Tx`, `TxOptions`, `Batch`, `Notification`

**Functions:**
- `New(ctx context.Context, cfg Config, pools ...NamedPool) (*Client, error)`
- `QueryInto[T any](ctx context.Context, c *Client, b *qb.Builder) ([]T, error)`
- `QueryOneInto[T any](ctx context.Context, c *Client, b *qb.Builder) (*T, error)`
- `InsertInto[T any](ctx context.Context, c *Client, b *qb.Builder, data map[string]any) (*T, error)`
- `UpdateInto[T any](ctx context.Context, c *Client, b *qb.Builder, data map[string]any) (*T, error)`
- `TxQueryInto[T any](ctx context.Context, tx pgx.Tx, b *qb.Builder) ([]T, error)`
- `TxQueryOneInto[T any](ctx context.Context, tx pgx.Tx, b *qb.Builder) (*T, error)`
- `TxInsertInto[T any](ctx context.Context, tx pgx.Tx, b *qb.Builder, data map[string]any) (*T, error)`
- `TxUpdateInto[T any](ctx context.Context, tx pgx.Tx, b *qb.Builder, data map[string]any) (*T, error)`
- `ScanUUID(row pgx.Row) (uuid.UUID, error)`
- `AcquireAdvisoryLock(ctx context.Context, tx Tx, key int64) error`
- `TryAdvisoryLock(ctx context.Context, tx Tx, key int64) (bool, error)`
- `IsNoRows(err error) bool`
- `IsUniqueViolation(err error) bool`
- `IsForeignKeyViolation(err error) bool`
- `IsNotNullViolation(err error) bool`
- `IsCheckViolation(err error) bool`
- `IsDeadlock(err error) bool`
- `IsSerializationFailure(err error) bool`
- `IsInvalidTextRepresentation(err error) bool`
- `IsUndefinedTable(err error) bool`
- `IsConnectionException(err error) bool`
- `PgError(err error) (*pgconn.PgError, bool)`

**Client Methods:**
- `Pool(name string) *pgxpool.Pool`
- `QB(table string) *qb.Builder`
- `Query(ctx context.Context, b *qb.Builder) ([]map[string]any, error)`
- `QueryOne(ctx context.Context, b *qb.Builder) (map[string]any, error)`
- `QueryWrite(ctx context.Context, b *qb.Builder) ([]map[string]any, error)`
- `QueryPool(ctx context.Context, poolName string, b *qb.Builder) ([]map[string]any, error)`
- `Insert(ctx context.Context, b *qb.Builder, data map[string]any) (uuid.UUID, error)`
- `InsertBatch(ctx context.Context, b *qb.Builder, rows []map[string]any) (uuid.UUID, error)`
- `Update(ctx context.Context, b *qb.Builder, data map[string]any) (int64, error)`
- `Delete(ctx context.Context, b *qb.Builder) (int64, error)`
- `ExecSQL(ctx context.Context, sql string, args ...any) (int64, error)`
- `ExecPoolSQL(ctx context.Context, poolName string, sql string, args ...any) (int64, error)`
- `QuerySQL(ctx context.Context, sql string, args ...any) ([]map[string]any, error)`
- `HealthCheck(ctx context.Context) error`
- `Close()`
- `WithTx(ctx context.Context, fn func(Tx) error) error`
- `WithTxOpts(ctx context.Context, opts TxOptions, fn func(Tx) error) error`
- `WithPoolTx(ctx context.Context, poolName string, fn func(Tx) error) error`
- `WithRetryTx(ctx context.Context, maxRetries int, fn func(Tx) error) error`
- `SendWrite(ctx context.Context, b *Batch) ([][]map[string]any, error)`
- `SendRead(ctx context.Context, b *Batch) ([][]map[string]any, error)`
- `Notify(ctx context.Context, channel, payload string) error`
- `Listen(ctx context.Context, channel string, handler func(Notification) error) error`
- `ListenMulti(ctx context.Context, channels []string, handler func(Notification) error) error`

**Batch Methods:**
- `NewBatch() *Batch`
- `Add(sql string, args ...any) *Batch`
- `AddExec(sql string, args ...any) *Batch`
- `AddSelect(builder *qb.Builder) *Batch`
- `AddInsert(builder *qb.Builder, data map[string]any) *Batch`
- `AddUpdate(builder *qb.Builder, data map[string]any) *Batch`
- `AddDelete(builder *qb.Builder) *Batch`
- `Len() int`

**Tx Interface Methods:**
- `QB(table string) *qb.Builder`
- `Insert(ctx context.Context, b *qb.Builder, data map[string]any) (uuid.UUID, error)`
- `Update(ctx context.Context, b *qb.Builder, data map[string]any) (int64, error)`
- `Delete(ctx context.Context, b *qb.Builder) (int64, error)`
- `Select(ctx context.Context, b *qb.Builder) ([]map[string]any, error)`
- `SelectOne(ctx context.Context, b *qb.Builder) (map[string]any, error)`
- `ExecRaw(ctx context.Context, sql string, args ...any) (int64, error)`
- `QueryRaw(ctx context.Context, sql string, args ...any) ([]map[string]any, error)`
- `Savepoint(ctx context.Context, name string) error`
- `RollbackTo(ctx context.Context, name string) error`
- `ReleaseSavepoint(ctx context.Context, name string) error`

**Errors:** `ErrNoRows`, `ErrEmptyRows`
