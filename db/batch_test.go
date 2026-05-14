package db

import (
	"strings"
	"testing"

	"github.com/rajangupta9/pgkit/qb"
)

func TestNewBatch_Empty(t *testing.T) {
	b := NewBatch()
	if b.Len() != 0 {
		t.Errorf("Len: got %d, want 0", b.Len())
	}
	if b.err != nil {
		t.Errorf("err: got %v, want nil", b.err)
	}
}

func TestBatch_Add_RawSelect(t *testing.T) {
	b := NewBatch().Add("SELECT 1")
	if b.Len() != 1 {
		t.Fatalf("Len: got %d", b.Len())
	}
	if !b.entries[0].returnsRows {
		t.Error("Add: expected returnsRows=true")
	}
	if b.entries[0].sql != "SELECT 1" {
		t.Errorf("sql: got %q", b.entries[0].sql)
	}
}

func TestBatch_AddExec_DoesNotExpectRows(t *testing.T) {
	b := NewBatch().AddExec("UPDATE x SET y = $1", 1)
	if b.entries[0].returnsRows {
		t.Error("AddExec: expected returnsRows=false")
	}
}

func TestBatch_AddSelect_BuildsAndStoresSQL(t *testing.T) {
	b := NewBatch().AddSelect(qb.New("users").Where(qb.Where("id", qb.OpEq, 7)))
	if b.err != nil {
		t.Fatalf("unexpected build error: %v", b.err)
	}
	if b.Len() != 1 {
		t.Fatalf("Len: got %d, want 1", b.Len())
	}
	if !strings.Contains(b.entries[0].sql, `FROM "users"`) {
		t.Errorf("sql missing FROM clause: %s", b.entries[0].sql)
	}
	if !b.entries[0].returnsRows {
		t.Error("expected returnsRows=true for SELECT")
	}
}

func TestBatch_AddSelect_NilBuilder_SetsError(t *testing.T) {
	b := NewBatch().AddSelect(nil)
	if b.err == nil || !strings.Contains(b.err.Error(), "nil builder") {
		t.Errorf("expected nil-builder error, got %v", b.err)
	}
}

func TestBatch_AddInsert_DefaultsToReturningID(t *testing.T) {
	b := NewBatch().AddInsert(qb.New("orders"), map[string]any{"total": 99})
	if b.err != nil {
		t.Fatalf("unexpected error: %v", b.err)
	}
	if !b.entries[0].returnsRows {
		t.Error("INSERT … RETURNING should set returnsRows=true")
	}
	if !strings.Contains(b.entries[0].sql, `RETURNING "id"`) {
		t.Errorf("expected RETURNING id by default, got: %s", b.entries[0].sql)
	}
}

func TestBatch_AddInsert_NilBuilder(t *testing.T) {
	b := NewBatch().AddInsert(nil, map[string]any{"x": 1})
	if b.err == nil {
		t.Error("expected error for nil builder")
	}
}

func TestBatch_AddInsert_EmptyData_RecordsError(t *testing.T) {
	b := NewBatch().AddInsert(qb.New("orders"), nil)
	if b.err == nil {
		t.Error("expected error for empty data map")
	}
}

func TestBatch_AddUpdate_WithoutReturning(t *testing.T) {
	b := NewBatch().AddUpdate(
		qb.New("users").Where(qb.Where("id", qb.OpEq, 1)),
		map[string]any{"name": "alice"},
	)
	if b.err != nil {
		t.Fatalf("unexpected error: %v", b.err)
	}
	if b.entries[0].returnsRows {
		t.Error("UPDATE without RETURNING should not expect rows")
	}
}

func TestBatch_AddUpdate_WithReturning(t *testing.T) {
	b := NewBatch().AddUpdate(
		qb.New("users").Where(qb.Where("id", qb.OpEq, 1)).Returning("id", "name"),
		map[string]any{"name": "alice"},
	)
	if b.err != nil {
		t.Fatalf("unexpected error: %v", b.err)
	}
	if !b.entries[0].returnsRows {
		t.Error("UPDATE … RETURNING should expect rows")
	}
}

func TestBatch_AddUpdate_NilBuilder(t *testing.T) {
	if b := NewBatch().AddUpdate(nil, map[string]any{"x": 1}); b.err == nil {
		t.Error("expected error for nil builder")
	}
}

func TestBatch_AddDelete_NoReturning(t *testing.T) {
	b := NewBatch().AddDelete(qb.New("users").Where(qb.Where("id", qb.OpEq, 1)))
	if b.err != nil {
		t.Fatalf("unexpected error: %v", b.err)
	}
	if b.entries[0].returnsRows {
		t.Error("DELETE without RETURNING should not expect rows")
	}
}

func TestBatch_AddDelete_WithReturning(t *testing.T) {
	b := NewBatch().AddDelete(
		qb.New("users").Where(qb.Where("id", qb.OpEq, 1)).ReturningAll(),
	)
	if !b.entries[0].returnsRows {
		t.Error("DELETE … RETURNING * should expect rows")
	}
}

func TestBatch_AddDelete_NilBuilder(t *testing.T) {
	if b := NewBatch().AddDelete(nil); b.err == nil {
		t.Error("expected error for nil builder")
	}
}

func TestBatch_FirstErrorWins(t *testing.T) {
	b := NewBatch().
		AddSelect(nil).               // first error
		AddInsert(qb.New("x"), nil) // second error — should be ignored
	if b.err == nil {
		t.Fatal("expected first error to be retained")
	}
	if !strings.Contains(b.err.Error(), "Select") {
		t.Errorf("expected first error to mention Select, got: %v", b.err)
	}
}

func TestBatch_MixedAddsCountCorrectly(t *testing.T) {
	b := NewBatch().
		Add("SELECT 1").
		AddExec("UPDATE x SET y=$1", 2).
		AddSelect(qb.New("users")).
		AddInsert(qb.New("orders"), map[string]any{"total": 1}).
		AddDelete(qb.New("logs"))
	if b.err != nil {
		t.Fatalf("unexpected error: %v", b.err)
	}
	if b.Len() != 5 {
		t.Errorf("Len: got %d, want 5", b.Len())
	}
}

func TestBatch_AddSelect_ChainPreservesFirstError(t *testing.T) {
	first := NewBatch().AddSelect(nil).err
	chained := NewBatch().AddSelect(nil).AddSelect(nil).err
	if first == nil || chained == nil {
		t.Fatal("expected errors from nil-builder selects")
	}
	if first.Error() != chained.Error() {
		t.Errorf("first error must win:\nfirst:   %v\nchained: %v", first, chained)
	}
}
