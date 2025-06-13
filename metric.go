package main

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
)

type (
	metrics struct {
		getCmd     int64
		getMissCmd int64
		putCmd     int64
		closeCmd   int64
		errors     int64
		Storage
	}
)

func NewStat(storage Storage) Storage {
	return &metrics{Storage: storage}
}

func (s *metrics) Get(ctx context.Context, key string) (GetResponse, bool, error) {
	atomic.AddInt64(&s.getCmd, 1)
	entry, ok, err := s.Storage.Get(ctx, key)
	if !ok {
		atomic.AddInt64(&s.getMissCmd, 1)
	}
	if err != nil {
		atomic.AddInt64(&s.errors, 1)
	}
	return entry, ok, err
}

func (s *metrics) Put(ctx context.Context, request PutRequest) (string, error) {
	atomic.AddInt64(&s.putCmd, 1)
	path, err := s.Storage.Put(ctx, request)
	if err != nil {
		atomic.AddInt64(&s.errors, 1)
	}
	return path, err
}

func (s *metrics) Close(ctx context.Context) error {
	err := s.Storage.Close(ctx)
	atomic.AddInt64(&s.closeCmd, 1)
	if err != nil {
		atomic.AddInt64(&s.errors, 1)
	}
	fmt.Fprintf(os.Stderr, "getCmd=%d putCmd=%d closeCmd=%d getMissCmd=%d errors=%d\n",
		atomic.LoadInt64(&s.getCmd),
		atomic.LoadInt64(&s.putCmd),
		atomic.LoadInt64(&s.closeCmd),
		atomic.LoadInt64(&s.getMissCmd),
		atomic.LoadInt64(&s.errors),
	)
	return err
}
