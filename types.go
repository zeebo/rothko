// Copyright (C) 2017. See AUTHORS.

package rothko

import (
	"context"

	"github.com/spacemonkeygo/rothko/data"
)

type Writer interface {
	Queue(ctx context.Context, series data.Series, record *data.Record) (
		err error)
}

type Source interface {
	Query(ctx context.Context, series data.Series, start, end int64) (
		Iterator, error)
	QueryLatest(ctx context.Context, series data.Series) (
		[]byte, error)

	Applications(ctx context.Context) (Iterator, error)
	Metrics(ctx context.Context, application string) (Iterator, error)
}

// Iterator is an iterator over a list of bytes or strings, like a
// https://golang.org/pkg/bufio/#Scanner.
type Iterator interface {
	Next(ctx context.Context) bool
	Bytes(ctx context.Context) []byte
	String(ctx context.Context) string
	Err(ctx context.Context) error
}