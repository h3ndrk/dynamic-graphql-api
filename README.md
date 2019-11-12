# Dynamic GraphQL API

This repository contains a dynamic relay-compliant GraphQL API server based on runtime database schema. This means that at startup the server queries the database schema, generates and connects the corresponding GraphQL types and resolvers with the database to create a Create/Read/Update/Delete (CRUD) interface to the database. The purpose of this repository is to simplify the repetitive process of defining and programming basic CRUD resolvers for a database.

The project is currently WIP as only the basic concept is implemented (working CRUD for an SQLite database). The roadmap contains filters, sorting, subscriptions, authentication, better logging and more.
