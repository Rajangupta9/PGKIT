// Package qb is a standalone PostgreSQL query builder.
//
// qb has no runtime dependency on a database connection — it only uses pgx
// for safe identifier quoting. Import it independently of any pool code.
//
// # Building Queries
//
// All builder methods return the same pointer for chaining. Errors accumulate
// and are returned on the first Build* call:
//
//	sql, args, err := qb.New("students").
//	    Columns("id", "name").
//	    Where(qb.Where("school_id", qb.OpEq, schoolID)).
//	    Where(qb.WhereExists(
//	        qb.New("enrolments").Where(qb.Where("student_id", qb.OpEq, studentID)),
//	    )).
//	    OrderBy("name", qb.Asc, qb.NullsLast).
//	    Limit(20).
//	    BuildSelect()
//
// # Supported Statements
//
//   - [Builder.BuildSelect]: SELECT with WHERE, JOIN, GROUP BY, HAVING, ORDER BY,
//     LIMIT, OFFSET, locking, UNION, UNION ALL, CTEs, window functions
//   - [Builder.BuildInsert]: INSERT … VALUES … ON CONFLICT … RETURNING
//   - [Builder.BuildInsertBatch]: multi-row INSERT in one statement
//   - [Builder.BuildUpdate]: UPDATE … SET … WHERE … RETURNING
//   - [Builder.BuildDelete]: DELETE … WHERE … RETURNING
//
// # Condition Constructors
//
// Use the Where* helpers instead of raw Condition literals:
//
//	qb.Where("price", qb.OpGt, 100)
//	qb.WhereIn("status", []string{"active", "pending"})
//	qb.WhereNull("deleted_at")
//	qb.WhereTextSearch("search_vector", "go & postgres")
//	qb.WhereJSONContains("metadata", map[string]any{"plan": "pro"})
//	qb.WhereExists(qb.New("orders").Where(qb.Where("user_id", qb.OpEq, uid)))
//	qb.WhereRaw("lower(email) = ?", email)
//
// # Security
//
// Column names passed to [Builder.Columns] and Where* helpers are quoted via
// [QuoteIdent]. Raw expressions (e.g. [Builder.Having], JOIN conditions,
// [Builder.OnConflict]) are written verbatim — never interpolate user input
// into those fields.
//
// # File Layout
//
//	types.go     — enum-like constants (JoinType, SortDir, LockMode, …)
//	builder.go   — Builder struct, New, Clone, lifecycle
//	quote.go     — identifier sanitisation (security-critical)
//	params.go    — placeholder helpers ($N shifting, ? injection)
//	select.go    — Columns/Distinct/WindowCol + BuildSelect
//	insert.go    — OnConflict + BuildInsert + BuildInsertBatch
//	update.go    — BuildUpdate
//	delete.go    — BuildDelete
//	where.go     — Where, WhereGroup, condition rendering
//	join.go      — INNER/LEFT/RIGHT/FULL/LATERAL joins
//	cte.go       — With, WithRecursive
//	union.go     — Union, UnionAll
//	clauses.go   — GROUP BY, ORDER BY, LIMIT, OFFSET, locking, RETURNING setters
//	condition.go — Operator constants + Condition constructors
package qb
