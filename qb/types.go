package qb

// JoinType is the SQL JOIN variant.
type JoinType string

const (
	JoinInner JoinType = "INNER"
	JoinLeft  JoinType = "LEFT"
	JoinRight JoinType = "RIGHT"
	JoinFull  JoinType = "FULL"
	JoinCross JoinType = "CROSS"
)

// SortDir is the ORDER BY direction.
type SortDir string

const (
	Asc  SortDir = "ASC"
	Desc SortDir = "DESC"
)

// NullsOrder controls NULL placement in ORDER BY.
type NullsOrder string

const (
	NullsFirst NullsOrder = "NULLS FIRST"
	NullsLast  NullsOrder = "NULLS LAST"
)

// LockMode controls row-level locking appended to SELECT.
type LockMode string

const (
	LockForUpdate      LockMode = "FOR UPDATE"
	LockForShare       LockMode = "FOR SHARE"
	LockForNoKeyUpdate LockMode = "FOR NO KEY UPDATE"
	LockForKeyShare    LockMode = "FOR KEY SHARE"
)

// LockWait controls wait behaviour when a lock cannot be acquired.
type LockWait string

const (
	Wait       LockWait = ""
	NoWait     LockWait = "NOWAIT"
	SkipLocked LockWait = "SKIP LOCKED"
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
