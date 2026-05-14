package db

import (
	"context"
	"fmt"
	"net"
)

// hostResolver looks up addresses for a host. Defaults to net.DefaultResolver.LookupHost
// in production; overridable for testing.
type hostResolver func(ctx context.Context, host string) ([]string, error)

var defaultHostResolver hostResolver = net.DefaultResolver.LookupHost

// preferIPv4Lookup returns IPv4 addresses first, IPv6 as fallback.
// pgx tries each in order — this prefers IPv4 (required on GCP Cloud Run)
// while still working when only AAAA records exist (local dev with Neon/Supabase).
func preferIPv4Lookup(ctx context.Context, host string) ([]string, error) {
	return preferIPv4LookupWith(ctx, host, defaultHostResolver)
}

func preferIPv4LookupWith(ctx context.Context, host string, resolve hostResolver) ([]string, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []string{host}, nil
	}
	all, err := resolve(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("db: DNS lookup %s: %w", host, err)
	}
	v4, v6 := splitByFamily(all)
	return append(v4, v6...), nil
}

// ipv4OnlyLookup fails hard if no A record exists.
// Avoids a slow IPv6 timeout on VMs without an IPv6 internet route.
func ipv4OnlyLookup(ctx context.Context, host string) ([]string, error) {
	return ipv4OnlyLookupWith(ctx, host, defaultHostResolver)
}

func ipv4OnlyLookupWith(ctx context.Context, host string, resolve hostResolver) ([]string, error) {
	if ip := net.ParseIP(host); ip != nil {
		if ip.To4() != nil {
			return []string{host}, nil
		}
		return nil, fmt.Errorf("db: IPv6 literal %s rejected (ForceIPv4=true)", host)
	}
	all, err := resolve(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("db: DNS lookup %s: %w", host, err)
	}
	v4, _ := splitByFamily(all)
	if len(v4) == 0 {
		return nil, fmt.Errorf("db: no IPv4 address for %s (got %v)", host, all)
	}
	return v4, nil
}

// splitByFamily partitions addrs into IPv4 and IPv6 buckets, preserving order.
// Non-parseable entries are silently dropped (mirrors net.LookupHost semantics).
func splitByFamily(addrs []string) (v4, v6 []string) {
	for _, a := range addrs {
		ip := net.ParseIP(a)
		if ip == nil {
			continue
		}
		if ip.To4() != nil {
			v4 = append(v4, a)
		} else {
			v6 = append(v6, a)
		}
	}
	return v4, v6
}
