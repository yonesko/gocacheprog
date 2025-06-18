package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sync/atomic"
	"time"
)

type (
	metrics struct {
		GetCmd        int64         `json:"gets"`
		GetMissCmd    int64         `json:"gets_miss"`
		PutCmd        int64         `json:"puts"`
		CloseCmd      int64         `json:"close"`
		Errors        int64         `json:"errors"`
		GetCmdMinTime time.Duration `json:"getCmdMinTime"`
		GetCmdAvgTime int64         `json:"getCmdAvgTime"`
		GetCmdMaxTime time.Duration `json:"getCmdMaxTime"`
		PutCmdMinTime time.Duration `json:"putCmdMinTime"`
		PutCmdAvgTime int64         `json:"putCmdAvgTime"`
		PutCmdMaxTime time.Duration `json:"putCmdMaxTime"`
		GetCmdTimeSum time.Duration `json:"-"`
		PutCmdTimeSum time.Duration `json:"-"`
		Storage       `json:"-"`
	}
)

func NewStat(storage Storage) Storage {
	return &metrics{
		GetCmd:        0,
		GetMissCmd:    0,
		PutCmd:        0,
		CloseCmd:      0,
		Errors:        0,
		GetCmdMinTime: math.MaxInt64,
		GetCmdAvgTime: 0,
		GetCmdMaxTime: math.MinInt64,
		PutCmdMinTime: math.MaxInt64,
		PutCmdAvgTime: 0,
		PutCmdMaxTime: math.MinInt64,
		GetCmdTimeSum: 0,
		PutCmdTimeSum: 0,
		Storage:       storage,
	}
}

func (s *metrics) Get(ctx context.Context, key string) (GetResponse, bool, error) {
	atomic.AddInt64(&s.GetCmd, 1)
	now := time.Now()
	entry, ok, err := s.Storage.Get(ctx, key)
	if !ok {
		atomic.AddInt64(&s.GetMissCmd, 1)
	}
	if err != nil {
		atomic.AddInt64(&s.Errors, 1)
	}
	elapsed := time.Since(now)
	s.GetCmdTimeSum += elapsed
	s.GetCmdMinTime = min(s.GetCmdMinTime, elapsed)
	s.GetCmdMaxTime = min(s.GetCmdMaxTime, elapsed)
	return entry, ok, err
}

func (s *metrics) Put(ctx context.Context, request PutRequest) (string, error) {
	atomic.AddInt64(&s.PutCmd, 1)
	now := time.Now()
	path, err := s.Storage.Put(ctx, request)
	if err != nil {
		atomic.AddInt64(&s.Errors, 1)
	}
	elapsed := time.Since(now)
	s.PutCmdTimeSum += elapsed
	s.PutCmdMinTime = min(s.PutCmdMinTime, elapsed)
	s.PutCmdMaxTime = min(s.PutCmdMaxTime, elapsed)
	return path, err
}

func (s *metrics) Close(ctx context.Context) error {
	err := s.Storage.Close(ctx)
	atomic.AddInt64(&s.CloseCmd, 1)
	if err != nil {
		atomic.AddInt64(&s.Errors, 1)
	}
	s.PutCmdAvgTime = int64(s.PutCmdTimeSum) / s.PutCmd
	s.GetCmdAvgTime = int64(s.GetCmdAvgTime) / s.GetCmd
	fmt.Fprintln(os.Stderr, string(must(json.Marshal(s))))
	return err
}
