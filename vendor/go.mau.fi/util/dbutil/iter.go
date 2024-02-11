// Copyright (c) 2023 Sumner Evans
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dbutil

// RowIter is a wrapper for [Rows] that allows conveniently iterating over rows
// with a predefined scanner function.
type RowIter[T any] interface {
	// Iter iterates over the rows and calls the given function for each row.
	//
	// If the function returns false, the iteration is stopped.
	// If the function returns an error, the iteration is stopped and the error is
	// returned.
	Iter(func(T) (bool, error)) error

	// AsList collects all rows into a slice.
	AsList() ([]T, error)
}

type rowIterImpl[T any] struct {
	Rows
	ConvertRow func(Scannable) (T, error)
}

// NewRowIter creates a new RowIter from the given Rows and scanner function.
func NewRowIter[T any](rows Rows, convertFn func(Scannable) (T, error)) RowIter[T] {
	return &rowIterImpl[T]{Rows: rows, ConvertRow: convertFn}
}

func ScanSingleColumn[T any](rows Scannable) (val T, err error) {
	err = rows.Scan(&val)
	return
}

type NewableDataStruct[T any] interface {
	DataStruct[T]
	New() T
}

func ScanDataStruct[T NewableDataStruct[T]](rows Scannable) (T, error) {
	var val T
	return val.New().Scan(rows)
}

func (i *rowIterImpl[T]) Iter(fn func(T) (bool, error)) error {
	if i == nil || i.Rows == nil {
		return nil
	}
	defer i.Rows.Close()

	for i.Rows.Next() {
		if item, err := i.ConvertRow(i.Rows); err != nil {
			return err
		} else if cont, err := fn(item); err != nil {
			return err
		} else if !cont {
			break
		}
	}
	return i.Rows.Err()
}

func (i *rowIterImpl[T]) AsList() (list []T, err error) {
	err = i.Iter(func(item T) (bool, error) {
		list = append(list, item)
		return true, nil
	})
	return
}
