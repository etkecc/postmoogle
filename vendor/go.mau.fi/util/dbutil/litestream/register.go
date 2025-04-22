//go:build cgo

package litestream

import (
	"database/sql"
	"database/sql/driver"

	"github.com/mattn/go-sqlite3"
)

var Functions = make(map[string]any)

func init() {
	sql.Register("litestream", &sqlite3.SQLiteDriver{
		ConnectHook: func(conn *sqlite3.SQLiteConn) (err error) {
			for name, fn := range Functions {
				err = conn.RegisterFunc(name, fn, true)
				if err != nil {
					return
				}
			}
			err = DoSetTrace(conn)
			if err != nil {
				return
			}
			if err = conn.SetFileControlInt("main", sqlite3.SQLITE_FCNTL_PERSIST_WAL, 1); err != nil {
				return
			}
			if _, err = conn.Exec("PRAGMA foreign_keys = ON", []driver.Value{}); err != nil {
				return
			}
			if _, err = conn.Exec("PRAGMA journal_mode = WAL", []driver.Value{}); err != nil {
				return
			}
			if _, err = conn.Exec("PRAGMA wal_autocheckpoint = 0", []driver.Value{}); err != nil {
				return
			}
			if _, err = conn.Exec("PRAGMA synchronous = NORMAL", []driver.Value{}); err != nil {
				return
			}
			if _, err = conn.Exec("PRAGMA busy_timeout = 5000", []driver.Value{}); err != nil {
				return
			}
			return
		},
	})

	sql.Register("sqlite3-fk-wal", &sqlite3.SQLiteDriver{
		ConnectHook: func(conn *sqlite3.SQLiteConn) (err error) {
			for name, fn := range Functions {
				err = conn.RegisterFunc(name, fn, true)
				if err != nil {
					return
				}
			}
			err = DoSetTrace(conn)
			if err != nil {
				return
			}
			if _, err = conn.Exec("PRAGMA foreign_keys = ON", []driver.Value{}); err != nil {
				return
			}
			if _, err = conn.Exec("PRAGMA journal_mode = WAL", []driver.Value{}); err != nil {
				return
			}
			if _, err = conn.Exec("PRAGMA synchronous = NORMAL", []driver.Value{}); err != nil {
				return
			}
			if _, err = conn.Exec("PRAGMA busy_timeout = 5000", []driver.Value{}); err != nil {
				return
			}
			return
		},
	})
}
