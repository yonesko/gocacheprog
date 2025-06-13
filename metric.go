package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync/atomic"
)

type (
	metrics struct {
		GetCmd     int64 `json:"gets"`
		GetMissCmd int64 `json:"gets_miss"`
		PutCmd     int64 `json:"puts"`
		CloseCmd   int64 `json:"close"`
		Errors     int64 `json:"errors"`
		Storage    `json:"-"`
	}
)

func NewStat(storage Storage) Storage {
	return &metrics{Storage: storage}
}

func (s *metrics) Get(ctx context.Context, key string) (GetResponse, bool, error) {
	atomic.AddInt64(&s.GetCmd, 1)
	entry, ok, err := s.Storage.Get(ctx, key)
	if !ok {
		atomic.AddInt64(&s.GetMissCmd, 1)
	}
	if err != nil {
		atomic.AddInt64(&s.Errors, 1)
	}
	return entry, ok, err
}

func (s *metrics) Put(ctx context.Context, request PutRequest) (string, error) {
	atomic.AddInt64(&s.PutCmd, 1)
	path, err := s.Storage.Put(ctx, request)
	if err != nil {
		atomic.AddInt64(&s.Errors, 1)
	}
	return path, err
}

func (s *metrics) Close(ctx context.Context) error {
	err := s.Storage.Close(ctx)
	atomic.AddInt64(&s.CloseCmd, 1)
	if err != nil {
		atomic.AddInt64(&s.Errors, 1)
	}
	fmt.Fprintln(os.Stderr, string(must(json.Marshal(s))))
	return err
}
