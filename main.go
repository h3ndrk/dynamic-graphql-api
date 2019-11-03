package main

import (
	"dynamic-graphql-api/handler"
	"log"
	"net/http"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	h, err := handler.NewHandler("sqlite3", "test.db")
	if err != nil {
		panic(err)
	}
	defer h.Close()

	http.Handle("/graphql", h)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
