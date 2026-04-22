package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"reflect"
	"sync"
	"sync/atomic"
	"text/tabwriter"
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

	// Initialize tabwriter
	w := tabwriter.NewWriter(os.Stderr, 0, 0, 2, ' ', 0)

	// Print overall stats table
	fmt.Fprintln(w, "=== OVERALL STATS ===")
	fmt.Fprintln(w, "Metric\tValue")
	fmt.Fprintln(w, "------\t-----")
	fmt.Fprintf(w, "Measured Storage\t%s\n", s.DecoratedName)
	fmt.Fprintf(w, "GET Operations\t%d\n", s.GetCmd)
	fmt.Fprintf(w, "GET Misses\t%d\n", s.GetMissCmd)
	fmt.Fprintf(w, "PUT Operations\t%d\n", s.PutCmd)
	fmt.Fprintf(w, "Close Operations\t%d\n", s.CloseCmd)
	fmt.Fprintf(w, "Errors\t%d\n", s.Errors)
	fmt.Fprintln(w, "")

	// Print GET operations stats table
	fmt.Fprintln(w, "=== GET OPERATIONS ===")
	fmt.Fprintln(w, "Metric\tValue\t")
	fmt.Fprintln(w, "------\t-----\t")
	if s.GetCmd > 0 {
		fmt.Fprintf(w, "Min Time\t%s\n", time.Duration(s.GetCmdMinTime).String())
		fmt.Fprintf(w, "Max Time\t%s\n", time.Duration(s.GetCmdMaxTime).String())
		fmt.Fprintf(w, "Avg Time\t%s\n", time.Duration(s.GetCmdAvgTime).String())
		fmt.Fprintf(w, "Total Time\t%s\n", time.Duration(s.GetCmdTimeSum).String())
	} else {
		fmt.Fprintln(w, "Min Time\tN/A")
		fmt.Fprintln(w, "Max Time\tN/A")
		fmt.Fprintln(w, "Avg Time\tN/A")
		fmt.Fprintln(w, "Total Time\tN/A")
	}
	fmt.Fprintln(w, "")

	// Print PUT operations stats table
	fmt.Fprintln(w, "=== PUT OPERATIONS ===")
	fmt.Fprintln(w, "Metric\tValue\t")
	fmt.Fprintln(w, "------\t-----\t")
	if s.PutCmd > 0 {
		fmt.Fprintf(w, "Min Time\t%s\n", time.Duration(s.PutCmdMinTime).String())
		fmt.Fprintf(w, "Max Time\t%s\n", time.Duration(s.PutCmdMaxTime).String())
		fmt.Fprintf(w, "Avg Time\t%s\n", time.Duration(s.PutCmdAvgTime).String())
		fmt.Fprintf(w, "Total Time\t%s\n", time.Duration(s.PutCmdTimeSum).String())
		fmt.Fprintf(w, "Min Size\t%s\n", humanSize(s.PutMinSize))
		fmt.Fprintf(w, "Max Size\t%s\n", humanSize(s.PutMaxSize))
		fmt.Fprintf(w, "Avg Size\t%s\n", humanSize(safeDiv(s.PutTotalSize, s.PutCmd)))
		fmt.Fprintf(w, "Total Size\t%s\n", humanSize(s.PutTotalSize))
	} else {
		fmt.Fprintln(w, "Min Time\tN/A")
		fmt.Fprintln(w, "Max Time\tN/A")
		fmt.Fprintln(w, "Avg Time\tN/A")
		fmt.Fprintln(w, "Total Time\tN/A")
		fmt.Fprintln(w, "Min Size\tN/A")
		fmt.Fprintln(w, "Max Size\tN/A")
		fmt.Fprintln(w, "Avg Size\tN/A")
		fmt.Fprintln(w, "Total Size\tN/A")
	}

	// Flush the writer to ensure all data is written
	w.Flush()

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
