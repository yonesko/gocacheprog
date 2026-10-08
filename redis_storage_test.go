package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func getRedisClient(tb testing.TB) redis.UniversalClient {
	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs: []string{"localhost:6379"},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		tb.Skipf("skipping: Redis/Valkey not available on localhost:6379: %v", err)
	}
	return client
}

type nonClosingClient struct {
	redis.UniversalClient
}

func (c nonClosingClient) Close() error {
	return nil
}

func Test_RedisStorage_RoundTrip(t *testing.T) {
	client := getRedisClient(t)
	defer client.Close()

	ctx := context.Background()
	storage := NewRedisStorage(client, fmt.Sprintf("test-%d", time.Now().UnixNano()))
	defer storage.Close(ctx)

	key := "test-key-1"
	bodyData := []byte("hello valkey cache")
	outputID := []byte("output-1")

	_, err := storage.Put(ctx, PutRequest{
		Key:      key,
		OutputID: outputID,
		Body:     bytes.NewReader(bodyData),
		BodySize: int64(len(bodyData)),
	})
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	resp, ok, err := storage.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected cache hit, got miss")
	}
	gotBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if !bytes.Equal(gotBody, bodyData) {
		t.Fatalf("body mismatch: got %q, want %q", gotBody, bodyData)
	}
}

// Benchmark_RedisStorage_Get benchmarks reading artifacts from Redis (tests strings.NewReader optimization).
func Benchmark_RedisStorage_Get(b *testing.B) {
	client := getRedisClient(b)
	defer client.Close()

	ctx := context.Background()
	prefix := fmt.Sprintf("bench-get-%d", time.Now().UnixNano())
	storage := NewRedisStorage(client, prefix)
	defer storage.Close(ctx)

	// Pre-populate artifacts of typical Go package sizes (100KB)
	const dataSize = 100 * 1024
	data := make([]byte, dataSize)
	rand.Read(data)

	key := "artifact-100k"
	_, err := storage.Put(ctx, PutRequest{
		Key:      key,
		OutputID: []byte("out1"),
		Body:     bytes.NewReader(data),
		BodySize: int64(len(data)),
	})
	if err != nil {
		b.Fatalf("seed Put: %v", err)
	}

	b.SetBytes(int64(dataSize))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		resp, ok, err := storage.Get(ctx, key)
		if err != nil || !ok {
			b.Fatalf("Get failed: %v (ok=%v)", err, ok)
		}
		// Read body to simulate consumer consumption
		_, _ = io.Copy(io.Discard, resp.Body)
	}
}

// Benchmark_RedisStorage_Compress benchmarks round-trip through zstd compression + Redis.
// This directly measures the sync.Pool reuse benefit for zstd.
func Benchmark_RedisStorage_Compress_RoundTrip(b *testing.B) {
	client := getRedisClient(b)
	defer client.Close()

	ctx := context.Background()
	prefix := fmt.Sprintf("bench-comp-%d", time.Now().UnixNano())
	storage := NewCompressStorage(NewRedisStorage(client, prefix))
	defer storage.Close(ctx)

	const dataSize = 100 * 1024
	// Generate moderately compressible text/binary data (typical for compiled code/AST)
	data := make([]byte, dataSize)
	for i := range data {
		data[i] = byte(i%26 + 'a')
	}

	b.SetBytes(int64(dataSize))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("item-%d", i%50) // 50 rotating keys to avoid infinite keyspace
		_, err := storage.Put(ctx, PutRequest{
			Key:      key,
			OutputID: []byte("out"),
			Body:     bytes.NewReader(data),
			BodySize: int64(len(data)),
		})
		if err != nil {
			b.Fatalf("Put failed: %v", err)
		}
		resp, ok, err := storage.Get(ctx, key)
		if err != nil || !ok {
			b.Fatalf("Get failed: %v (ok=%v)", err, ok)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
	}
}

// Benchmark_App_Redis simulates the full build pipeline with DecoratorStorage (local FS + Redis + Compress)
func Benchmark_App_Redis(b *testing.B) {
	client := getRedisClient(b)
	defer client.Close()

	const (
		numActions   = 200
		hitRatio     = 0.70
		numPreSeeded = int(numActions * hitRatio) // 140 cached, 60 fresh
	)

	sizeOf := func(i int) int {
		switch {
		case i%10 == 0:
			return 1_000_000
		case i%10 < 4:
			return 300_000
		default:
			return 50_000
		}
	}

	type action struct {
		actionID []byte
		outputID []byte
		body     []byte
	}

	actions := make([]action, numActions)
	totalBytes := int64(0)
	for i := range actions {
		size := sizeOf(i)
		body := make([]byte, size)
		for j := range body {
			body[j] = byte(j%26 + 'A') // compressible
		}
		actions[i] = action{
			actionID: []byte(fmt.Sprintf("act%04d", i)),
			outputID: []byte(fmt.Sprintf("out%04d", i)),
			body:     body,
		}
		totalBytes += int64(size)
	}

	stream := func() string {
		buf := &bytes.Buffer{}
		enc := json.NewEncoder(buf)
		id := int64(1)
		for i, a := range actions {
			enc.Encode(Request{
				ID:       id,
				Command:  CmdGet,
				ActionID: a.actionID,
			})
			id++
			if i >= numPreSeeded {
				enc.Encode(Request{
					ID:       id,
					Command:  CmdPut,
					ActionID: a.actionID,
					OutputID: a.outputID,
					BodySize: int64(len(a.body)),
				})
				id++
				enc.Encode(a.body)
			}
		}
		enc.Encode(Request{ID: id, Command: CmdClose})
		return buf.String()
	}()

	ctx := context.Background()
	b.SetBytes(totalBytes)
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()

		dir1 := must(os.MkdirTemp("", "gocacheprog-bench-redis-*"))
		prefix := fmt.Sprintf("bench-app-%d-%d", time.Now().UnixNano(), i)
		extStorage := NewCompressStorage(NewRedisStorage(nonClosingClient{client}, prefix))
		storage := NewDecoratorStorage(NewFileSystemStorage(dir1), extStorage)

		// seed the warm portion of the cache into Redis
		for j := 0; j < numPreSeeded; j++ {
			a := actions[j]
			storage.Put(ctx, PutRequest{
				Key:      hex.EncodeToString(a.actionID),
				OutputID: a.outputID,
				Body:     bytes.NewReader(a.body),
				BodySize: int64(len(a.body)),
			})
		}

		app := NewApp(strings.NewReader(stream), io.Discard, hex.EncodeToString, storage)
		b.StartTimer()

		app.Run(ctx)

		b.StopTimer()
		os.RemoveAll(dir1)
	}
}
