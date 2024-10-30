package websql

import (
	"fmt"
	"log"
	"testing"
)

func TestWebSQL(t *testing.T) {

	SetDBDir("./")

	DeleteAllDatabases()
	dbId, _, err := Open("mydb", "", false)
	if err != nil {
		log.Fatal(err)
	}
	txId, err := BeginTransaction(dbId)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("create table\n")
	insertId, rowsAffected, data, err := Exec(txId, `CREATE TABLE mytable (
	id INTEGER PRIMARY KEY,
	name TEXT default ""
)`, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("insertId=%v rowsAffected=%v, data=%v\n", insertId, rowsAffected, data)

	fmt.Printf("insert hello\n")
	insertId, rowsAffected, data, err = Exec(txId, "INSERT INTO mytable (id, name) VALUES (?, ?)", []interface{}{0, "hello"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("insertId=%v rowsAffected=%v, data=%v\n", insertId, rowsAffected, data)

	fmt.Printf("commit\n")
	err = Commit(txId)
	if err != nil {
		log.Fatal(err)
	}

	txId, err = BeginTransaction(dbId)
	fmt.Printf("BEGIN txid=%v\n", txId)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("insert hello2\n")
	insertId, rowsAffected, data, err = Exec(txId, "INSERT INTO mytable (id, name) VALUES (?, ?)", []interface{}{1, "hello2"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("insertId=%v rowsAffected=%v, data=%v\n", insertId, rowsAffected, data)

	fmt.Printf("select\n")
	insertId, rowsAffected, data, err = Exec(txId, "SELECT * FROM mytable", []interface{}{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("insertId=%v rowsAffected=%v, data=%v\n", insertId, rowsAffected, data)

	Commit(txId)

	txId, err = BeginTransaction(dbId)
	fmt.Printf("BEGIN txid=%v\n", txId)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("select\n")
	insertId, rowsAffected, data, err = Exec(txId, "SELECT * FROM mytable", []interface{}{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("insertId=%v rowsAffected=%v, data=%v\n", insertId, rowsAffected, data)
	Commit(txId)

	fmt.Printf("test OK\n")
}
