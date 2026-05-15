package db_test

import (
	"fmt"

	"github.com/rajangupta9/pgkit/db"
	"github.com/rajangupta9/pgkit/qb"
)

func ExampleClient_QB() {
	c := &db.Client{}
	b := c.QB("orders").Columns("id", "amount").Where(qb.Where("status", qb.OpEq, "active"))

	fmt.Println(b != nil)

	// Output:
	// true
}
