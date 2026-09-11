package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"math/big"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"
)

// compressStorage wraps an external storage (e.g. Redis) that always populates
// GetResponse.Body. fileSystemStorage returns only DiskPath and is never used
// directly under compressStorage. Tests use a mock inner storage to control
// what Get returns.

// newRoundTripMock builds a mock that stores whatever Put gives it and hands
// it back on Get, simulating a body-returning backend like Redis. Shared
// between tests and benchmarks.
func newRoundTripMock(ctrl *gomock.Controller) *MockStorage {
	inner := NewMockStorage(ctrl)
	var storedBody []byte
	var storedOutputID []byte
	inner.EXPECT().Put(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req PutRequest) (string, error) {
			var err error
			storedBody, err = io.ReadAll(req.Body)
			storedOutputID = req.OutputID
			return "", err
		}).AnyTimes()
	inner.EXPECT().Get(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string) (GetResponse, bool, error) {
			if storedBody == nil {
				return GetResponse{}, false, nil
			}
			return GetResponse{
				OutputID: storedOutputID,
				Body:     bytes.NewReader(storedBody),
				BodySize: int64(len(storedBody)),
			}, true, nil
		}).AnyTimes()
	inner.EXPECT().Close(gomock.Any()).Return(nil).AnyTimes()
	return inner
}

func Test_CompressStorage(t *testing.T) {
	ctx := context.Background()

	t.Run("put then get returns same body", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		storage := NewCompressStorage(newRoundTripMock(ctrl))

		const body = "hello compress storage"
		_, err := storage.Put(ctx, PutRequest{
			Key:      "testkey",
			OutputID: []byte("out1"),
			Body:     strings.NewReader(body),
			BodySize: int64(len(body)),
		})
		if err != nil {
			t.Fatalf("Put: %v", err)
		}

		resp, ok, err := storage.Get(ctx, "testkey")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if !ok {
			t.Fatal("expected cache hit, got miss")
		}
		got, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if string(got) != body {
			t.Fatalf("body mismatch: want %q, got %q", body, string(got))
		}
		if string(resp.OutputID) != "out1" {
			t.Fatalf("OutputID mismatch: want %q, got %q", "out1", string(resp.OutputID))
		}
		_ = storage.Close(ctx)
	})

	t.Run("get miss propagates correctly", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		inner := NewMockStorage(ctrl)
		inner.EXPECT().Get(gomock.Any(), "nonexistent").Return(GetResponse{}, false, nil).Times(1)
		inner.EXPECT().Close(gomock.Any()).Return(nil).Times(1)

		storage := NewCompressStorage(inner)
		_, ok, err := storage.Get(ctx, "nonexistent")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			t.Fatal("expected miss, got hit")
		}
		_ = storage.Close(ctx)
	})

	t.Run("get error from inner storage propagates", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		inner := NewMockStorage(ctrl)
		inner.EXPECT().Get(gomock.Any(), "k").Return(GetResponse{}, false, io.ErrUnexpectedEOF).Times(1)
		inner.EXPECT().Close(gomock.Any()).Return(nil).Times(1)

		storage := NewCompressStorage(inner)
		_, _, err := storage.Get(ctx, "k")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		_ = storage.Close(ctx)
	})

	t.Run("put compresses: inner receives different bytes than original", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		inner := NewMockStorage(ctrl)
		// highly compressible body so we can be sure compression actually shrinks it
		body := strings.Repeat("a", 1024)

		var capturedBody []byte
		inner.EXPECT().Put(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, req PutRequest) (string, error) {
				var err error
				capturedBody, err = io.ReadAll(req.Body)
				return "", err
			}).Times(1)
		inner.EXPECT().Close(gomock.Any()).Return(nil).Times(1)

		storage := NewCompressStorage(inner)
		_, err := storage.Put(ctx, PutRequest{
			Key:      "k",
			OutputID: []byte("o"),
			Body:     strings.NewReader(body),
			BodySize: int64(len(body)),
		})
		if err != nil {
			t.Fatalf("Put: %v", err)
		}
		if bytes.Equal(capturedBody, []byte(body)) {
			t.Fatal("expected compressed data, inner storage received plaintext")
		}
		if len(capturedBody) >= len(body) {
			t.Fatalf("expected compression to reduce size: original=%d, compressed=%d", len(body), len(capturedBody))
		}
		_ = storage.Close(ctx)
	})

	t.Run("large body round-trip", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		storage := NewCompressStorage(newRoundTripMock(ctrl))

		body := must(randomString(64 * 1024))
		_, err := storage.Put(ctx, PutRequest{
			Key:      "bigkey",
			OutputID: []byte("bigout"),
			Body:     strings.NewReader(body),
			BodySize: int64(len(body)),
		})
		if err != nil {
			t.Fatalf("Put: %v", err)
		}
		resp, ok, err := storage.Get(ctx, "bigkey")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if !ok {
			t.Fatal("expected hit")
		}
		got, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if string(got) != body {
			t.Fatalf("large body mismatch (len want=%d got=%d)", len(body), len(got))
		}
		_ = storage.Close(ctx)
	})

	t.Run("empty body round-trip", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		storage := NewCompressStorage(newRoundTripMock(ctrl))

		_, err := storage.Put(ctx, PutRequest{
			Key:      "emptykey",
			OutputID: []byte("emptyout"),
			Body:     strings.NewReader(""),
			BodySize: 0,
		})
		if err != nil {
			t.Fatalf("Put: %v", err)
		}
		resp, ok, err := storage.Get(ctx, "emptykey")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if !ok {
			t.Fatal("expected hit")
		}
		got, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("expected empty body, got %q", got)
		}
		_ = storage.Close(ctx)
	})
}

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func randomString(length int) (string, error) {
	result := make([]byte, length)
	for i := range result {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[num.Int64()]
	}
	return string(result), nil
}

func Benchmark_CompressStorage(b *testing.B) {
	ctx := context.Background()

	sizes := []struct {
		name string
		size int
	}{
		{"1KB", 1 << 10},
		{"64KB", 64 << 10},
		{"1MB", 1 << 20},
	}

	for _, s := range sizes {
		body := strings.Repeat("x", s.size) // highly compressible
		req := PutRequest{
			Key:      "benchkey",
			OutputID: []byte("out"),
			BodySize: int64(s.size),
		}

		b.Run("put/"+s.name, func(b *testing.B) {
			ctrl := gomock.NewController(b)
			storage := NewCompressStorage(newRoundTripMock(ctrl))
			b.SetBytes(int64(s.size))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				req.Body = strings.NewReader(body)
				if _, err := storage.Put(ctx, req); err != nil {
					b.Fatal(err)
				}
			}
			storage.Close(ctx)
		})

		b.Run("get/"+s.name, func(b *testing.B) {
			ctrl := gomock.NewController(b)
			storage := NewCompressStorage(newRoundTripMock(ctrl))
			req.Body = strings.NewReader(body)
			if _, err := storage.Put(ctx, req); err != nil {
				b.Fatal(err)
			}
			b.SetBytes(int64(s.size))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				resp, ok, err := storage.Get(ctx, "benchkey")
				if err != nil || !ok {
					b.Fatalf("get failed: ok=%v err=%v", ok, err)
				}
				io.Copy(io.Discard, resp.Body) //nolint
			}
			storage.Close(ctx)
		})
	}
}
