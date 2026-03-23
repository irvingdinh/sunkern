---
title: Avoid Common SQL Mistakes
impact: HIGH
impactDescription: prevents connection and data issues
tags: sql, database, prepared-statements, null
---

## Avoid Common SQL Mistakes

**Impact: HIGH (prevents connection and data issues)**

Four common SQL gotchas in Go:

**A) `sql.Open` may not actually connect.** It only validates the DSN format. The first real connection happens on the first query, which can fail unexpectedly in production. Always call `db.Ping()` after opening.

**B) Configure the connection pool.** The defaults allow unlimited open connections, which can exhaust database resources under load.

**C) Use prepared statements for repeated queries.** They reduce parsing overhead and protect against SQL injection.

**D) Handle NULL values.** A `NULL` column scanned into a `string` or `int` will cause a runtime error. Use `sql.NullString`, `sql.NullInt64`, or pointers.

**Incorrect (what's wrong):**

```go
// A) sql.Open may not connect
db, err := sql.Open("postgres", dsn)
if err != nil {
	return err
}
// Start serving requests — may fail on first query

// B) No connection pool configuration — defaults allow unlimited connections

// C) Query repeated in a loop without preparation
for _, id := range ids {
	db.QueryRow("SELECT name FROM users WHERE id = $1", id)
}

// D) NULL not handled
var name string
err := db.QueryRow("SELECT nickname FROM users WHERE id = $1", id).Scan(&name)
// Crashes if nickname is NULL
```

**Correct (what's right):**

```go
// A) Verify connectivity with Ping
db, err := sql.Open("postgres", dsn)
if err != nil {
	return err
}
if err := db.Ping(); err != nil {
	return fmt.Errorf("connecting to db: %w", err)
}

// B) Configure connection pool for production
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(25)
db.SetConnMaxIdleTime(5 * time.Minute)

// C) Use prepared statements for repeated queries
stmt, err := db.Prepare("SELECT name FROM users WHERE id = $1")
if err != nil {
	return err
}
defer stmt.Close()
for _, id := range ids {
	stmt.QueryRow(id)
}

// D) Handle NULL with sql.NullString or pointers
var name sql.NullString
err := db.QueryRow("SELECT nickname FROM users WHERE id = $1", id).Scan(&name)
if err != nil {
	return err
}
if name.Valid {
	fmt.Println(name.String)
}
```
