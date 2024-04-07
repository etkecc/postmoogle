// Copyright (c) 2023 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dbutil

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/rs/zerolog"

	"go.mau.fi/util/exerrors"
	"go.mau.fi/util/random"
)

var (
	ErrTxn       = errors.New("transaction")
	ErrTxnBegin  = fmt.Errorf("%w: begin", ErrTxn)
	ErrTxnCommit = fmt.Errorf("%w: commit", ErrTxn)
)

type contextKey int

const (
	ContextKeyDatabaseTransaction contextKey = iota
	ContextKeyDoTxnCallerSkip
)

func (db *Database) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return db.Conn(ctx).ExecContext(ctx, query, args...)
}

func (db *Database) Query(ctx context.Context, query string, args ...any) (Rows, error) {
	return db.Conn(ctx).QueryContext(ctx, query, args...)
}

func (db *Database) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return db.Conn(ctx).QueryRowContext(ctx, query, args...)
}

func (db *Database) BeginTx(ctx context.Context, opts *sql.TxOptions) (*LoggingTxn, error) {
	if ctx == nil {
		panic("BeginTx() called with nil ctx")
	}
	return db.LoggingDB.BeginTx(ctx, opts)
}

func (db *Database) DoTxn(ctx context.Context, opts *sql.TxOptions, fn func(ctx context.Context) error) error {
	if ctx == nil {
		panic("DoTxn() called with nil ctx")
	}
	if ctx.Value(ContextKeyDatabaseTransaction) != nil {
		zerolog.Ctx(ctx).Trace().Msg("Already in a transaction, not creating a new one")
		return fn(ctx)
	}

	log := zerolog.Ctx(ctx).With().Str("db_txn_id", random.String(12)).Logger()
	slowLog := log

	callerSkip := 1
	if val := ctx.Value(ContextKeyDoTxnCallerSkip); val != nil {
		callerSkip += val.(int)
	}
	if pc, file, line, ok := runtime.Caller(callerSkip); ok {
		slowLog = log.With().Str(zerolog.CallerFieldName, zerolog.CallerMarshalFunc(pc, file, line)).Logger()
	}

	start := time.Now()
	deadlockCh := make(chan struct{})
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				slowLog.Warn().
					Dur("duration_seconds", time.Since(start)).
					Msg("Transaction still running")
			case <-deadlockCh:
				return
			}
		}
	}()
	defer func() {
		close(deadlockCh)
		dur := time.Since(start)
		if dur > time.Second {
			slowLog.Warn().
				Float64("duration_seconds", dur.Seconds()).
				Msg("Transaction took long")
		}
	}()
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		log.Trace().Err(err).Msg("Failed to begin transaction")
		return exerrors.NewDualError(ErrTxnBegin, err)
	}
	log.Trace().Msg("Transaction started")
	tx.noTotalLog = true
	ctx = log.WithContext(ctx)
	ctx = context.WithValue(ctx, ContextKeyDatabaseTransaction, tx)
	err = fn(ctx)
	if err != nil {
		log.Trace().Err(err).Msg("Database transaction failed, rolling back")
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			log.Warn().Err(rollbackErr).Msg("Rollback after transaction error failed")
		} else {
			log.Trace().Msg("Rollback successful")
		}
		return err
	}
	err = tx.Commit()
	if err != nil {
		log.Trace().Err(err).Msg("Commit failed")
		return exerrors.NewDualError(ErrTxnCommit, err)
	}
	log.Trace().Msg("Commit successful")
	return nil
}

func (db *Database) Conn(ctx context.Context) Execable {
	if ctx == nil {
		panic("Conn() called with nil ctx")
	}
	txn, ok := ctx.Value(ContextKeyDatabaseTransaction).(Transaction)
	if ok {
		return txn
	}
	return &db.LoggingDB
}
