package db

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolConfig holds settings for a single named connection pool.
// ConnString is required; all other fields have built-in defaults.
type PoolConfig struct {
	ConnString        string
	MaxConns          int32
	MinConns          int32
	MaxConnIdleTime   time.Duration
	MaxConnLifetime   time.Duration
	HealthCheckPeriod time.Duration
	ConnectTimeout    time.Duration
	ForceIPv4         bool
}

func (c *PoolConfig) applyDefaults() {
	if c.MaxConns == 0 {
		c.MaxConns = 10
	}
	if c.MinConns == 0 {
		c.MinConns = 2
	}
	if c.MaxConnIdleTime == 0 {
		c.MaxConnIdleTime = 5 * time.Minute
	}
	if c.MaxConnLifetime == 0 {
		c.MaxConnLifetime = 1 * time.Hour
	}
	if c.HealthCheckPeriod == 0 {
		c.HealthCheckPeriod = 1 * time.Minute
	}
	if c.ConnectTimeout == 0 {
		c.ConnectTimeout = 20 * time.Second
	}
}

// NamedPool pairs a name with its pool configuration.
// Each NamedPool can have completely independent credentials and DSN.
type NamedPool struct {
	Name string
	PoolConfig
}

type poolManager struct {
	mu    sync.RWMutex
	pools map[string]*pgxpool.Pool
}

// Get returns the *pgxpool.Pool registered under name, or nil if not found.
func (m *poolManager) Get(name string) *pgxpool.Pool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pools[name]
}

func (m *poolManager) HealthCheck(ctx context.Context) error {
	m.mu.RLock()
	names := make([]string, 0, len(m.pools))
	pools := make([]*pgxpool.Pool, 0, len(m.pools))
	for name, p := range m.pools {
		names = append(names, name)
		pools = append(pools, p)
	}
	m.mu.RUnlock()

	type result struct {
		name string
		err  error
	}
	ch := make(chan result, len(pools))
	for i, p := range pools {
		go func(name string, pool *pgxpool.Pool) {
			if err := pool.Ping(ctx); err != nil {
				ch <- result{name, err}
			} else {
				ch <- result{name, nil}
			}
		}(names[i], p)
	}

	var errs []error
	for range pools {
		if r := <-ch; r.err != nil {
			errs = append(errs, fmt.Errorf("pool %s: %w", r.name, r.err))
		}
	}
	return errors.Join(errs...)
}

func (m *poolManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.pools {
		p.Close()
	}
}

func newPoolManager(ctx context.Context, namedPools []NamedPool) (*poolManager, error) {
	if err := validateNamedPools(namedPools); err != nil {
		return nil, err
	}

	type buildResult struct {
		name string
		pool *pgxpool.Pool
		err  error
	}

	built := make(map[string]*pgxpool.Pool, len(namedPools))
	results := make(chan buildResult, len(namedPools))

	for _, np := range namedPools {
		go func(np NamedPool) {
			p, err := buildPool(ctx, np.PoolConfig, np.Name)
			results <- buildResult{name: np.Name, pool: p, err: err}
		}(np)
	}

	var errs []error
	for range namedPools {
		r := <-results
		if r.err != nil {
			errs = append(errs, fmt.Errorf("db: pool %q: %w", r.name, r.err))
			continue
		}
		built[r.name] = r.pool
	}

	if len(errs) > 0 {
		closeAll(built)
		return nil, errors.Join(errs...)
	}

	return &poolManager{pools: built}, nil
}

// validateNamedPools checks the input list for structural errors before any
// network calls. Keeping this separate from buildPool means duplicates and
// blank fields are caught instantly, without burning a TCP connection on
// the first pool only to discover the second is broken.
func validateNamedPools(namedPools []NamedPool) error {
	if len(namedPools) == 0 {
		return errors.New("db: at least one NamedPool is required")
	}
	seen := make(map[string]struct{}, len(namedPools))
	for _, np := range namedPools {
		if np.Name == "" {
			return errors.New("db: NamedPool.Name must not be empty")
		}
		if np.ConnString == "" {
			return fmt.Errorf("db: pool %q: ConnString is required", np.Name)
		}
		if _, dup := seen[np.Name]; dup {
			return fmt.Errorf("db: duplicate pool name %q", np.Name)
		}
		seen[np.Name] = struct{}{}
	}
	return nil
}

func closeAll(pools map[string]*pgxpool.Pool) {
	for _, p := range pools {
		p.Close()
	}
}

func buildPool(ctx context.Context, cfg PoolConfig, name string) (*pgxpool.Pool, error) {
	cfg.applyDefaults()

	pCfg, err := pgxpool.ParseConfig(cfg.ConnString)
	if err != nil {
		return nil, fmt.Errorf("parse config for %s pool: %w", name, err)
	}

	if cfg.ForceIPv4 {
		pCfg.ConnConfig.LookupFunc = ipv4OnlyLookup
	} else {
		pCfg.ConnConfig.LookupFunc = preferIPv4Lookup
	}

	pCfg.MaxConns = cfg.MaxConns
	pCfg.MinConns = cfg.MinConns
	pCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	pCfg.MaxConnLifetime = cfg.MaxConnLifetime
	pCfg.HealthCheckPeriod = cfg.HealthCheckPeriod
	pCfg.ConnConfig.ConnectTimeout = cfg.ConnectTimeout

	// SimpleProtocol disables prepared statements and binary encoding.
	// Required for compatibility with PgBouncer in transaction mode.
	// Slightly slower but avoids prepared-statement cache issues.
	pCfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	pool, err := pgxpool.NewWithConfig(ctx, pCfg)
	if err != nil {
		return nil, fmt.Errorf("create %s pool: %w", name, err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()
	if err = pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping %s pool: %w", name, err)
	}
	return pool, nil
}
