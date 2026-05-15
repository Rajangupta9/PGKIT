package qb

import "fmt"

// MaxQueryParams is the PostgreSQL limit on bound parameters per query.
const MaxQueryParams = 65535

var (
	// ErrNilBuilder is returned when a Builder method is called on a nil builder.
	ErrNilBuilder = fmt.Errorf("qb: builder is nil")

	// ErrNoTable is returned when the Builder has no target table.
	ErrNoTable = fmt.Errorf("qb: table name must not be empty")

	// ErrEmptyData is returned when INSERT or UPDATE data is empty.
	ErrEmptyData = fmt.Errorf("qb: data map must not be empty")

	// ErrEmptyRows is returned when batch rows are empty.
	ErrEmptyRows = fmt.Errorf("qb: batch rows must not be empty")
)

func tooManyParamsErr(got int) error {
	return fmt.Errorf("qb: query has %d parameters, exceeding PostgreSQL limit of %d", got, MaxQueryParams)
}
