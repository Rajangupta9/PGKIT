// Package db provides PostgreSQL connection pooling, query execution,
// transactions, typed scanning, batch dispatch, and LISTEN/NOTIFY support,
// built on top of [github.com/jackc/pgx/v5].
//
// Use [github.com/rajangupta9/pgkit/qb] standalone when you only need to
// build SQL strings. Use this package when you also need pool management,
// transactions, and execution.
//
// # Creating a Client
//
// Construct a [Client] once at startup with one or more named pools. By
// convention "write" targets the primary and "read" targets a replica, but
// any names work:
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
// # Querying
//
// Use [Client.QB] to get a query builder, then pass it to [Client.Query]:
//
//	rows, err := client.Query(ctx, client.QB("users").
//	    Columns("id", "name").
//	    Where(qb.Where("active", qb.OpEq, true)).
//	    Limit(20),
//	)
//
// # Typed Scanning
//
// Use the generic [QueryInto] and [InsertInto] functions for compile-time type safety:
//
//	type User struct {
//	    ID   uuid.UUID `db:"id"`
//	    Name string    `db:"name"`
//	}
//	users, err := db.QueryInto[User](ctx, client, client.QB("users").Limit(20))
//
// # Transactions
//
// [Client.WithTx] commits on nil return and rolls back otherwise:
//
//	err := client.WithTx(ctx, func(tx db.Tx) error {
//	    id, err := tx.Insert(ctx, tx.QB("orders"), data)
//	    return err
//	})
//
// # Batch Queries
//
// [Client.SendWrite] and [Client.SendRead] dispatch multiple queries in a
// single network round-trip:
//
//	b := db.NewBatch()
//	b.AddSelect(client.QB("users").Where(qb.Where("id", qb.OpEq, uid)))
//	b.AddExec("UPDATE sessions SET last_seen = NOW() WHERE user_id = $1", uid)
//	results, err := client.SendWrite(ctx, b)
//
// # Error Helpers
//
// PostgreSQL error codes are wrapped in typed predicates:
//
//	if db.IsUniqueViolation(err) { ... }
//	if db.IsSerializationFailure(err) { ... }
package db
