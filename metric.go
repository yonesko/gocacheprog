package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

type (
	metrics struct {
		DecoratedName string
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
		PutMinSize    int64
		PutMaxSize    int64
		PutTotalSize  int64
		sync.Mutex
		Storage
	}
)

func NewMetricsStorage(storage Storage) Storage {
	return &metrics{
		DecoratedName: reflect.TypeOf(storage).String(),
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
		PutMinSize:    math.MaxInt64,
		PutMaxSize:    math.MinInt64,
		PutTotalSize:  0,
		Storage:       storage,
	}
}

func (s *metrics) Get(ctx context.Context, key string) (GetResponse, bool, error) {
	s.Lock()
	defer s.Unlock()
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
	s.Lock()
	defer s.Unlock()
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
	s.PutMaxSize = max(s.PutMaxSize, request.BodySize)
	s.PutMinSize = min(s.PutMinSize, request.BodySize)
	s.PutTotalSize += request.BodySize
	return path, err
}

func (s *metrics) Close(ctx context.Context) error {
	s.Lock()
	defer s.Unlock()
	err := s.Storage.Close(ctx)
	atomic.AddInt64(&s.CloseCmd, 1)
	if err != nil {
		atomic.AddInt64(&s.Errors, 1)
	}
	s.PutCmdAvgTime = safeDiv(s.PutCmdTimeSum, s.PutCmd)
	s.GetCmdAvgTime = safeDiv(s.GetCmdTimeSum, s.GetCmd)

	stats := map[string]any{
		"measured":  s.DecoratedName,
		"gets":      s.GetCmd,
		"gets_miss": s.GetMissCmd,
		"puts":      s.PutCmd,
		"close":     s.CloseCmd,
		"errors":    s.Errors,

		"get_min_time": time.Duration(s.GetCmdMinTime).String(),
		"get_max_time": time.Duration(s.GetCmdMaxTime).String(),
		"get_avg_time": time.Duration(s.GetCmdAvgTime).String(),
		"get_sum_time": time.Duration(s.GetCmdTimeSum).String(),

		"put_min_time": time.Duration(s.PutCmdMinTime).String(),
		"put_max_time": time.Duration(s.PutCmdMaxTime).String(),
		"put_avg_time": time.Duration(s.PutCmdAvgTime).String(),
		"put_sum_time": time.Duration(s.PutCmdTimeSum).String(),

		"put_min_size": humanSize(s.PutMinSize),
		"put_avg_size": humanSize(safeDiv(s.PutTotalSize, s.PutCmd)),
		"put_max_size": humanSize(s.PutMaxSize),
		"put_sum_size": humanSize(s.PutTotalSize),
	}

	fmt.Fprint(os.Stderr, string(must(json.Marshal(stats))))
	return err
}

func humanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func safeDiv(a, b int64) int64 {
	if b == 0 {
		return -1
	}
	return a / b
}
