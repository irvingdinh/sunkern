package db

import (
	"context"
	"database/sql"
	"fmt"
)

// RunTx executes fn inside a database transaction. If fn returns nil, the
// transaction is committed. If fn returns an error or panics, the transaction
// is rolled back. On panic, the panic is re-raised after rollback.
//
// Pass the write pool (*sql.DB from sqlite.DB.WriteDB()) as the db argument.
// The *sql.Tx passed to fn satisfies Querier, so all query builders work
// unchanged inside the transaction.
func RunTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) (err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: begin tx: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	err = fn(tx)
	if err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("db: commit tx: %w", err)
	}

	return nil
}

// RunTxVal executes fn inside a database transaction and returns a value.
// If fn returns nil error, the transaction is committed and the value is
// returned. If fn returns an error or panics, the transaction is rolled back.
//
// Use this when the transaction needs to produce a result:
//
//	user, err := db.RunTxVal(ctx, writeDB, func(tx *sql.Tx) (User, error) {
//	    q := db.Insert(&Users.TableInfo).Model(&u).ReturningStar()
//	    created, err := db.Returning[User](ctx, tx, q)
//	    if err != nil {
//	        return User{}, err
//	    }
//	    // ... more operations in the same transaction ...
//	    return created, nil
//	})
func RunTxVal[T any](ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) (T, error)) (val T, err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return val, fmt.Errorf("db: begin tx: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	val, err = fn(tx)
	if err != nil {
		return val, err
	}

	if err = tx.Commit(); err != nil {
		return val, fmt.Errorf("db: commit tx: %w", err)
	}

	return val, nil
}
