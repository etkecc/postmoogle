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
)

type upgradeFunc func(context.Context, *Database) error

type upgrade struct {
	message string
	fn      upgradeFunc

	upgradesTo    int
	compatVersion int
	transaction   bool
}

var ErrUnsupportedDatabaseVersion = errors.New("unsupported database schema version")
var ErrForeignTables = errors.New("the database contains foreign tables")
var ErrNotOwned = errors.New("the database is owned by")
var ErrUnsupportedDialect = errors.New("unsupported database dialect")

func (db *Database) upgradeVersionTable(ctx context.Context) error {
	if compatColumnExists, err := db.ColumnExists(ctx, db.VersionTable, "compat"); err != nil {
		return fmt.Errorf("failed to check if version table is up to date: %w", err)
	} else if !compatColumnExists {
		if tableExists, err := db.TableExists(ctx, db.VersionTable); err != nil {
			return fmt.Errorf("failed to check if version table exists: %w", err)
		} else if !tableExists {
			_, err = db.Exec(ctx, fmt.Sprintf("CREATE TABLE %s (version INTEGER, compat INTEGER)", db.VersionTable))
			if err != nil {
				return fmt.Errorf("failed to create version table: %w", err)
			}
		} else {
			_, err = db.Exec(ctx, fmt.Sprintf("ALTER TABLE %s ADD COLUMN compat INTEGER", db.VersionTable))
			if err != nil {
				return fmt.Errorf("failed to add compat column to version table: %w", err)
			}
		}
	}
	return nil
}

func (db *Database) getVersion(ctx context.Context) (version, compat int, err error) {
	if err = db.upgradeVersionTable(ctx); err != nil {
		return
	}

	var compatNull sql.NullInt32
	err = db.QueryRow(ctx, fmt.Sprintf("SELECT version, compat FROM %s LIMIT 1", db.VersionTable)).Scan(&version, &compatNull)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	if compatNull.Valid && compatNull.Int32 != 0 {
		compat = int(compatNull.Int32)
	} else {
		compat = version
	}
	return
}

const (
	tableExistsPostgres = "SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name=$1)"
	tableExistsSQLite   = "SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND tbl_name=?1)"
)

func (db *Database) TableExists(ctx context.Context, table string) (exists bool, err error) {
	switch db.Dialect {
	case SQLite:
		err = db.QueryRow(ctx, tableExistsSQLite, table).Scan(&exists)
	case Postgres:
		err = db.QueryRow(ctx, tableExistsPostgres, table).Scan(&exists)
	default:
		err = ErrUnsupportedDialect
	}
	return
}

const (
	columnExistsPostgres = "SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name=$1 AND column_name=$2)"
	columnExistsSQLite   = "SELECT EXISTS(SELECT 1 FROM pragma_table_info(?1) WHERE name=?2)"
)

func (db *Database) ColumnExists(ctx context.Context, table, column string) (exists bool, err error) {
	switch db.Dialect {
	case SQLite:
		err = db.QueryRow(ctx, columnExistsSQLite, table, column).Scan(&exists)
	case Postgres:
		err = db.QueryRow(ctx, columnExistsPostgres, table, column).Scan(&exists)
	default:
		err = ErrUnsupportedDialect
	}
	return
}

func (db *Database) tableExistsNoError(ctx context.Context, table string) bool {
	exists, err := db.TableExists(ctx, table)
	if err != nil {
		panic(fmt.Errorf("failed to check if table exists: %w", err))
	}
	return exists
}

const createOwnerTable = `
CREATE TABLE IF NOT EXISTS database_owner (
	key   INTEGER PRIMARY KEY DEFAULT 0,
	owner TEXT NOT NULL
)
`

func (db *Database) checkDatabaseOwner(ctx context.Context) error {
	var owner string
	if !db.IgnoreForeignTables {
		if db.tableExistsNoError(ctx, "state_groups_state") {
			return fmt.Errorf("%w (found state_groups_state, likely belonging to Synapse)", ErrForeignTables)
		} else if db.tableExistsNoError(ctx, "roomserver_rooms") {
			return fmt.Errorf("%w (found roomserver_rooms, likely belonging to Dendrite)", ErrForeignTables)
		}
	}
	if db.Owner == "" {
		return nil
	}
	if _, err := db.Exec(ctx, createOwnerTable); err != nil {
		return fmt.Errorf("failed to ensure database owner table exists: %w", err)
	} else if err = db.QueryRow(ctx, "SELECT owner FROM database_owner WHERE key=0").Scan(&owner); errors.Is(err, sql.ErrNoRows) {
		_, err = db.Exec(ctx, "INSERT INTO database_owner (key, owner) VALUES (0, $1)", db.Owner)
		if err != nil {
			return fmt.Errorf("failed to insert database owner: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to check database owner: %w", err)
	} else if owner != db.Owner {
		return fmt.Errorf("%w %s", ErrNotOwned, owner)
	}
	return nil
}

func (db *Database) setVersion(ctx context.Context, version, compat int) error {
	_, err := db.Exec(ctx, fmt.Sprintf("DELETE FROM %s", db.VersionTable))
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, fmt.Sprintf("INSERT INTO %s (version, compat) VALUES ($1, $2)", db.VersionTable), version, compat)
	return err
}

func (db *Database) Upgrade(ctx context.Context) error {
	err := db.checkDatabaseOwner(ctx)
	if err != nil {
		return err
	}

	version, compat, err := db.getVersion(ctx)
	if err != nil {
		return err
	}

	if compat > len(db.UpgradeTable) {
		if db.IgnoreUnsupportedDatabase {
			db.Log.WarnUnsupportedVersion(version, compat, len(db.UpgradeTable))
			return nil
		}
		return fmt.Errorf("%w: currently on v%d (compatible down to v%d), latest known: v%d", ErrUnsupportedDatabaseVersion, version, compat, len(db.UpgradeTable))
	}

	db.Log.PrepareUpgrade(version, compat, len(db.UpgradeTable))
	logVersion := version
	for version < len(db.UpgradeTable) {
		upgradeItem := db.UpgradeTable[version]
		if upgradeItem.fn == nil {
			version++
			continue
		}
		doUpgrade := func(ctx context.Context) error {
			err = upgradeItem.fn(ctx, db)
			if err != nil {
				return fmt.Errorf("failed to run upgrade v%d->v%d: %w", version, upgradeItem.upgradesTo, err)
			}
			version = upgradeItem.upgradesTo
			logVersion = version
			err = db.setVersion(ctx, version, upgradeItem.compatVersion)
			if err != nil {
				return err
			}
			return nil
		}
		db.Log.DoUpgrade(logVersion, upgradeItem.upgradesTo, upgradeItem.message, upgradeItem.transaction)
		if upgradeItem.transaction {
			err = db.DoTxn(ctx, nil, doUpgrade)
		} else {
			err = doUpgrade(ctx)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
