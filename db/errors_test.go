package db

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func pgErr(code string) error {
	return &pgconn.PgError{Code: code}
}

func TestIsNoRows(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"local ErrNoRows", ErrNoRows, true},
		{"pgx ErrNoRows", pgx.ErrNoRows, true},
		{"wrapped ErrNoRows", fmt.Errorf("ctx: %w", ErrNoRows), true},
		{"wrapped pgx.ErrNoRows", fmt.Errorf("ctx: %w", pgx.ErrNoRows), true},
		{"unrelated error", errors.New("boom"), false},
		{"nil", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsNoRows(c.err); got != c.want {
				t.Errorf("IsNoRows(%v): got %v, want %v", c.err, got, c.want)
			}
		})
	}
}

func TestPgErrorPredicates(t *testing.T) {
	cases := []struct {
		name  string
		check func(error) bool
		code  string
		match bool
	}{
		{"UniqueViolation match", IsUniqueViolation, "23505", true},
		{"UniqueViolation miss", IsUniqueViolation, "23503", false},
		{"ForeignKeyViolation match", IsForeignKeyViolation, "23503", true},
		{"NotNullViolation match", IsNotNullViolation, "23502", true},
		{"CheckViolation match", IsCheckViolation, "23514", true},
		{"Deadlock match", IsDeadlock, "40P01", true},
		{"SerializationFailure match", IsSerializationFailure, "40001", true},
		{"InvalidTextRepresentation match", IsInvalidTextRepresentation, "22P02", true},
		{"UndefinedTable match", IsUndefinedTable, "42P01", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.check(pgErr(c.code)); got != c.match {
				t.Errorf("check on code %q: got %v, want %v", c.code, got, c.match)
			}
		})
	}
}

func TestIsConnectionException(t *testing.T) {
	cases := map[string]bool{
		"08000": true,
		"08003": true,
		"08006": true,
		"08P01": true,
		"23505": false,
		"":      false,
		"0":     false,
	}
	for code, want := range cases {
		if got := IsConnectionException(pgErr(code)); got != want {
			t.Errorf("IsConnectionException(%q): got %v, want %v", code, got, want)
		}
	}
}

func TestPredicates_NonPgErrorReturnsFalse(t *testing.T) {
	plain := errors.New("not a pg error")
	predicates := []func(error) bool{
		IsUniqueViolation, IsForeignKeyViolation, IsNotNullViolation,
		IsCheckViolation, IsDeadlock, IsSerializationFailure,
		IsInvalidTextRepresentation, IsUndefinedTable, IsConnectionException,
	}
	for _, p := range predicates {
		if p(plain) {
			t.Errorf("predicate returned true for plain error: %v", plain)
		}
		if p(nil) {
			t.Errorf("predicate returned true for nil error")
		}
	}
}

func TestPgError_Unwraps(t *testing.T) {
	inner := &pgconn.PgError{Code: "23505", Message: "dup"}
	wrapped := fmt.Errorf("db: insert scan: %w", inner)
	got, ok := PgError(wrapped)
	if !ok {
		t.Fatalf("PgError did not unwrap")
	}
	if got.Code != "23505" || got.Message != "dup" {
		t.Errorf("PgError returned wrong inner: %+v", got)
	}

	if _, ok := PgError(errors.New("plain")); ok {
		t.Error("PgError returned ok for non-pg error")
	}
	if _, ok := PgError(nil); ok {
		t.Error("PgError returned ok for nil")
	}
}
