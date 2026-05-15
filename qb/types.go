package qb

// JoinType is the SQL JOIN variant used by [Builder.Join] and its helpers.
type JoinType string

const (
	JoinInner JoinType = "INNER" // INNER JOIN — only matching rows from both sides.
	JoinLeft  JoinType = "LEFT"  // LEFT JOIN — all rows from the left, matched rows from the right.
	JoinRight JoinType = "RIGHT" // RIGHT JOIN — all rows from the right, matched rows from the left.
	JoinFull  JoinType = "FULL"  // FULL JOIN — all rows from both sides.
	JoinCross JoinType = "CROSS" // CROSS JOIN — cartesian product; no ON condition.
)

// SortDir is the ORDER BY direction used by [Builder.OrderBy].
type SortDir string

const (
	Asc  SortDir = "ASC"  // Ascending order (smallest first).
	Desc SortDir = "DESC" // Descending order (largest first).
)

// NullsOrder controls NULL placement in ORDER BY, passed as an optional
// third argument to [Builder.OrderBy].
type NullsOrder string

const (
	NullsFirst NullsOrder = "NULLS FIRST" // NULLs sort before all non-null values.
	NullsLast  NullsOrder = "NULLS LAST"  // NULLs sort after all non-null values.
)

// LockMode controls the row-level locking clause appended to a SELECT.
// Pass to [Builder.Lock]; or use the convenience methods [Builder.ForUpdate]
// and [Builder.ForShare].
type LockMode string

const (
	LockForUpdate      LockMode = "FOR UPDATE"        // Exclusive lock; blocks all other lock modes.
	LockForShare       LockMode = "FOR SHARE"         // Shared lock; allows other FOR SHARE locks.
	LockForNoKeyUpdate LockMode = "FOR NO KEY UPDATE" // Like FOR UPDATE but does not block FOR KEY SHARE.
	LockForKeyShare    LockMode = "FOR KEY SHARE"     // Minimal lock; only blocks FOR UPDATE.
)

// LockWait controls the wait behaviour when the desired row lock cannot be
// acquired immediately. Passed to [Builder.ForUpdate], [Builder.ForShare],
// and [Builder.Lock].
type LockWait string

const (
	Wait       LockWait = ""            // Block until the lock is available (default behaviour).
	NoWait     LockWait = "NOWAIT"      // Return an error immediately if the lock cannot be acquired.
	SkipLocked LockWait = "SKIP LOCKED" // Skip rows that are already locked rather than waiting.
)

// returningMode controls RETURNING clause rendering for INSERT/UPDATE/DELETE.
type returningMode int

const (
	returningUnset   returningMode = iota // not explicitly set — use the Build* default
	returningID                           // RETURNING "id"
	returningAll                          // RETURNING *
	returningColumns                      // RETURNING col1, col2, …
	returningNone                         // explicitly suppressed — no RETURNING clause
)
