package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync/atomic"
)

type (
	stat struct {
		getCmd   int64
		putCmd   int64
		closeCmd int64
		Storage
	}
)

func NewStat(storage Storage) Storage {
	return &stat{Storage: storage}
}

func (s stat) Get(ctx context.Context, key string) (Entry, bool, error) {
	atomic.AddInt64(&s.getCmd, 1)
	return s.Storage.Get(ctx, key)
}

func (s stat) Put(ctx context.Context, key string, outputID []byte, body io.Reader) (string, error) {
	atomic.AddInt64(&s.putCmd, 1)
	return s.Storage.Put(ctx, key, outputID, body)
}

func (s stat) Close(ctx context.Context) error {
	atomic.AddInt64(&s.closeCmd, 1)
	fmt.Fprintf(os.Stderr, "getCmd=%d putCmd=%d closeCmd=%d\n",
		atomic.LoadInt64(&s.getCmd),
		atomic.LoadInt64(&s.putCmd),
		atomic.LoadInt64(&s.closeCmd),
	)
	return s.Storage.Close(ctx)
}
