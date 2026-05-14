package db

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestPoolConfig_ApplyDefaults(t *testing.T) {
	c := PoolConfig{}
	c.applyDefaults()

	checks := []struct {
		name string
		got  any
		want any
	}{
		{"MaxConns", int(c.MaxConns), 10},
		{"MinConns", int(c.MinConns), 2},
		{"MaxConnIdleTime", c.MaxConnIdleTime, 5 * time.Minute},
		{"MaxConnLifetime", c.MaxConnLifetime, 1 * time.Hour},
		{"HealthCheckPeriod", c.HealthCheckPeriod, 1 * time.Minute},
		{"ConnectTimeout", c.ConnectTimeout, 20 * time.Second},
	}
	for _, tc := range checks {
		if tc.got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}

func TestPoolConfig_DefaultsDoNotOverrideExplicit(t *testing.T) {
	c := PoolConfig{
		MaxConns:          50,
		MinConns:          10,
		MaxConnIdleTime:   30 * time.Second,
		MaxConnLifetime:   2 * time.Hour,
		HealthCheckPeriod: 10 * time.Second,
		ConnectTimeout:    5 * time.Second,
	}
	c.applyDefaults()

	if c.MaxConns != 50 {
		t.Errorf("MaxConns overridden: got %d, want 50", c.MaxConns)
	}
	if c.MinConns != 10 {
		t.Errorf("MinConns overridden: got %d", c.MinConns)
	}
	if c.MaxConnIdleTime != 30*time.Second {
		t.Errorf("MaxConnIdleTime overridden")
	}
	if c.MaxConnLifetime != 2*time.Hour {
		t.Errorf("MaxConnLifetime overridden")
	}
	if c.HealthCheckPeriod != 10*time.Second {
		t.Errorf("HealthCheckPeriod overridden")
	}
	if c.ConnectTimeout != 5*time.Second {
		t.Errorf("ConnectTimeout overridden")
	}
}

func TestValidateNamedPools_RejectsEmptyList(t *testing.T) {
	if err := validateNamedPools(nil); err == nil {
		t.Fatal("expected error for empty pool list")
	}
	if err := validateNamedPools([]NamedPool{}); err == nil {
		t.Fatal("expected error for zero-length pool list")
	}
}

func TestValidateNamedPools_RejectsBlankName(t *testing.T) {
	err := validateNamedPools([]NamedPool{
		{Name: "", PoolConfig: PoolConfig{ConnString: "postgres://x"}},
	})
	if err == nil || !strings.Contains(err.Error(), "Name must not be empty") {
		t.Fatalf("expected blank-name error, got %v", err)
	}
}

func TestValidateNamedPools_RejectsMissingConnString(t *testing.T) {
	err := validateNamedPools([]NamedPool{
		{Name: "write"},
	})
	if err == nil || !strings.Contains(err.Error(), "ConnString is required") {
		t.Fatalf("expected ConnString error, got %v", err)
	}
}

func TestValidateNamedPools_RejectsDuplicateName(t *testing.T) {
	err := validateNamedPools([]NamedPool{
		{Name: "rw", PoolConfig: PoolConfig{ConnString: "postgres://localhost/x"}},
		{Name: "rw", PoolConfig: PoolConfig{ConnString: "postgres://localhost/y"}},
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate pool name") {
		t.Fatalf("expected duplicate-name error, got %v", err)
	}
}

func TestValidateNamedPools_Accepts_GoodInput(t *testing.T) {
	err := validateNamedPools([]NamedPool{
		{Name: "read", PoolConfig: PoolConfig{ConnString: "postgres://r"}},
		{Name: "write", PoolConfig: PoolConfig{ConnString: "postgres://w"}},
	})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestPoolManager_GetReturnsNilForMissingName(t *testing.T) {
	pm := &poolManager{pools: nil}
	if got := pm.Get("anything"); got != nil {
		t.Errorf("expected nil for missing pool, got %v", got)
	}
}

func TestPoolManager_HealthCheck_NoPoolsIsNoError(t *testing.T) {
	pm := &poolManager{pools: nil}
	if err := pm.HealthCheck(context.Background()); err != nil {
		t.Errorf("HealthCheck on empty pool manager: %v", err)
	}
}

func TestPoolManager_Close_NoPoolsIsSafe(t *testing.T) {
	pm := &poolManager{pools: nil}
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Close on empty pool manager panicked: %v", r)
		}
	}()
	pm.Close()
}

// Sanity: the package's exported sentinel errors are stable identities.
func TestSentinelErrors_DistinctIdentities(t *testing.T) {
	if errors.Is(ErrNoRows, ErrEmptyRows) {
		t.Error("ErrNoRows and ErrEmptyRows should not be Is-equal")
	}
}
