package db

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func staticResolver(addrs []string, err error) hostResolver {
	return func(_ context.Context, _ string) ([]string, error) {
		return addrs, err
	}
}

func TestSplitByFamily(t *testing.T) {
	v4, v6 := splitByFamily([]string{"1.2.3.4", "::1", "10.0.0.1", "fe80::1", "garbage"})
	wantV4 := []string{"1.2.3.4", "10.0.0.1"}
	wantV6 := []string{"::1", "fe80::1"}
	if !equalStringSlice(v4, wantV4) {
		t.Errorf("v4: got %v, want %v", v4, wantV4)
	}
	if !equalStringSlice(v6, wantV6) {
		t.Errorf("v6: got %v, want %v", v6, wantV6)
	}
}

func TestPreferIPv4_LiteralIP(t *testing.T) {
	got, err := preferIPv4LookupWith(context.Background(), "127.0.0.1", staticResolver(nil, errors.New("must not be called")))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !equalStringSlice(got, []string{"127.0.0.1"}) {
		t.Errorf("got %v, want [127.0.0.1]", got)
	}
}

func TestPreferIPv4_OrdersV4FirstThenV6(t *testing.T) {
	r := staticResolver([]string{"::1", "10.0.0.1", "fe80::1", "192.168.0.1"}, nil)
	got, err := preferIPv4LookupWith(context.Background(), "example.com", r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"10.0.0.1", "192.168.0.1", "::1", "fe80::1"}
	if !equalStringSlice(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestPreferIPv4_ResolverError(t *testing.T) {
	r := staticResolver(nil, errors.New("nxdomain"))
	_, err := preferIPv4LookupWith(context.Background(), "nope.invalid", r)
	if err == nil {
		t.Fatal("expected error from resolver, got nil")
	}
	if !strings.Contains(err.Error(), "DNS lookup nope.invalid") {
		t.Errorf("error missing host context: %v", err)
	}
}

func TestIPv4Only_LiteralV4Allowed(t *testing.T) {
	got, err := ipv4OnlyLookupWith(context.Background(), "10.0.0.1", staticResolver(nil, errors.New("must not be called")))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !equalStringSlice(got, []string{"10.0.0.1"}) {
		t.Errorf("got %v, want [10.0.0.1]", got)
	}
}

func TestIPv4Only_LiteralV6Rejected(t *testing.T) {
	_, err := ipv4OnlyLookupWith(context.Background(), "::1", staticResolver(nil, nil))
	if err == nil {
		t.Fatal("expected error for IPv6 literal, got nil")
	}
	if !strings.Contains(err.Error(), "IPv6 literal") {
		t.Errorf("error missing IPv6 context: %v", err)
	}
}

func TestIPv4Only_FailsWhenNoARecord(t *testing.T) {
	r := staticResolver([]string{"::1", "fe80::1"}, nil)
	_, err := ipv4OnlyLookupWith(context.Background(), "v6only.example", r)
	if err == nil {
		t.Fatal("expected error when no A record exists, got nil")
	}
	if !strings.Contains(err.Error(), "no IPv4 address") {
		t.Errorf("error missing v4-only context: %v", err)
	}
}

func TestIPv4Only_FiltersOutV6(t *testing.T) {
	r := staticResolver([]string{"::1", "10.0.0.1", "fe80::1", "192.168.0.1"}, nil)
	got, err := ipv4OnlyLookupWith(context.Background(), "example.com", r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"10.0.0.1", "192.168.0.1"}
	if !equalStringSlice(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
