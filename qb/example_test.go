package qb_test

import (
	"fmt"

	"github.com/rajangupta9/pgkit/qb"
)

func ExampleNew() {
	sql, args, err := qb.New("users").
		Columns("id", "name").
		Where(qb.Where("active", qb.OpEq, true)).
		OrderBy("created_at", qb.Desc).
		Limit(10).
		BuildSelect()
	if err != nil {
		return
	}

	fmt.Println(sql)
	fmt.Println(args)

	// Output:
	// SELECT "id", "name" FROM "users" WHERE "active" = $1 ORDER BY "created_at" DESC LIMIT 10
	// [true]
}
