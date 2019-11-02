package main

import (
	"database/sql"
	"dynamic-graphql-api/handler/schema"
	"log"
	"net/http"

	"github.com/graphql-go/handler"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "test.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	sqlRows, err := db.Query(
		"SELECT sql FROM sqlite_master WHERE type = 'table'",
	)
	if err != nil {
		panic(err)
	}
	defer sqlRows.Close()

	var (
		sqlString string
		sqls      []string
	)
	for sqlRows.Next() {
		err := sqlRows.Scan(&sqlString)
		if err != nil {
			panic(err)
		}

		sqls = append(sqls, sqlString)
	}

	if err := sqlRows.Err(); err != nil {
		panic(err)
	}

	s, err := schema.NewSchema(sqls)
	if err != nil {
		panic(err)
	}

	h := handler.New(&handler.Config{
		Schema:     s,
		Pretty:     true,
		Playground: true,
	})

	http.Handle("/graphql" /*httpDBMiddleware(db,*/, h /*)*/)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
