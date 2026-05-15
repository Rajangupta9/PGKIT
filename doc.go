// Package pgkit is a lightweight PostgreSQL toolkit for Go.
//
// PGKIT provides two complementary subpackages for building PostgreSQL
// applications: a standalone SQL query builder and a pgx-based database layer.
//
// # Subpackages
//
// [github.com/rajangupta9/pgkit/qb] is a standalone SQL query builder with no
// runtime connection dependency. Use it to construct parameterised queries:
//
//	sql, args, err := qb.New("users").
//	    Columns("id", "name", "email").
//	    Where(qb.Where("active", qb.OpEq, true)).
//	    OrderBy("created_at", qb.Desc).
//	    Limit(20).
//	    BuildSelect()
//
// [github.com/rajangupta9/pgkit/db] manages named connection pools and
// executes queries against PostgreSQL via pgx v5:
//
//	client, err := db.New(ctx, db.Config{},
//	    db.NamedPool{Name: "write", PoolConfig: db.PoolConfig{ConnString: writeDSN}},
//	    db.NamedPool{Name: "read",  PoolConfig: db.PoolConfig{ConnString: readDSN}},
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	rows, err := client.Query(ctx, client.QB("orders").
//	    Where(qb.Where("status", qb.OpEq, "paid")).
//	    Limit(50),
//	)
//
// # Installation
//
//	go get github.com/rajangupta9/pgkit
package pgkit
