package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

type (
	metrics struct {
		GetCmd        int64
		GetMissCmd    int64
		PutCmd        int64
		CloseCmd      int64
		Errors        int64
		GetCmdMinTime int64
		GetCmdAvgTime int64
		GetCmdMaxTime int64
		PutCmdMinTime int64
		PutCmdAvgTime int64
		PutCmdMaxTime int64
		GetCmdTimeSum int64
		PutCmdTimeSum int64
		Storage
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
	elapsed := int64(time.Since(now))
	s.GetCmdTimeSum += elapsed
	s.GetCmdMinTime = min(s.GetCmdMinTime, elapsed)
	s.GetCmdMaxTime = max(s.GetCmdMaxTime, elapsed)
	return entry, ok, err
}

func (s *metrics) Put(ctx context.Context, request PutRequest) (string, error) {
	atomic.AddInt64(&s.PutCmd, 1)
	now := time.Now()
	path, err := s.Storage.Put(ctx, request)
	if err != nil {
		atomic.AddInt64(&s.Errors, 1)
	}
	elapsed := int64(time.Since(now))
	s.PutCmdTimeSum += elapsed
	s.PutCmdMinTime = min(s.PutCmdMinTime, elapsed)
	s.PutCmdMaxTime = max(s.PutCmdMaxTime, elapsed)
	return path, err
}

func (s *metrics) Close(ctx context.Context) error {
	err := s.Storage.Close(ctx)
	atomic.AddInt64(&s.CloseCmd, 1)
	if err != nil {
		atomic.AddInt64(&s.Errors, 1)
	}
	s.PutCmdAvgTime = s.PutCmdTimeSum / s.PutCmd
	s.GetCmdAvgTime = s.GetCmdTimeSum / s.GetCmd
	fmt.Fprintf(os.Stderr, strings.Join([]string{
		"gets:%d",
		"gets_miss:%d",
		"puts:%d",
		"close:%d",
		"errors:%d",

		"get min time:%s",
		"get max time:%s",
		"get avg time:%s",
		"get sum time:%s",

		"put min time:%s",
		"put max time:%s",
		"put avg time:%s",
		"put sum time:%s",
	}, "\n")+"\n",
		s.GetCmd,
		s.GetMissCmd,
		s.PutCmd,
		s.CloseCmd,
		s.Errors,

		time.Duration(s.GetCmdMinTime),
		time.Duration(s.GetCmdMaxTime),
		time.Duration(s.GetCmdAvgTime),
		time.Duration(s.GetCmdTimeSum),

		time.Duration(s.PutCmdMinTime),
		time.Duration(s.PutCmdMaxTime),
		time.Duration(s.PutCmdAvgTime),
		time.Duration(s.PutCmdTimeSum),
	)
	return err
}
