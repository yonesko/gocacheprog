package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test_FileSystemStorage(t *testing.T) {
	ctx := context.Background()

	t.Run("put then get round-trip", func(t *testing.T) {
		s := NewFileSystemStorage(t.TempDir())
		const body = "hello filesystem"
		diskPath, err := s.Put(ctx, PutRequest{
			Key:      "key1",
			OutputID: []byte("out1"),
			Body:     strings.NewReader(body),
			BodySize: int64(len(body)),
		})
		assertNoErr(t, err)
		assertNotEmpty(t, diskPath, "expected diskPath")
		assertTrue(t, filepath.IsAbs(diskPath), "diskPath must be absolute")

		resp, ok, err := s.Get(ctx, "key1")
		assertNoErr(t, err)
		assertTrue(t, ok, "expected hit")
		assertEqual(t, "out1", string(resp.OutputID))
		assertEqual(t, diskPath, resp.DiskPath)
		assertEqual(t, int64(len(body)), resp.BodySize)

		// file on disk must contain the body
		data, err := os.ReadFile(diskPath)
		assertNoErr(t, err)
		assertEqual(t, body, string(data))
	})

	t.Run("get miss for unknown key", func(t *testing.T) {
		s := NewFileSystemStorage(t.TempDir())
		_, ok, err := s.Get(ctx, "nosuchkey")
		assertNoErr(t, err)
		assertFalse(t, ok, "expected miss")
	})

	t.Run("empty body round-trip", func(t *testing.T) {
		s := NewFileSystemStorage(t.TempDir())
		diskPath, err := s.Put(ctx, PutRequest{
			Key:      "empty",
			OutputID: []byte("emptyout"),
			Body:     bytes.NewReader(nil),
			BodySize: 0,
		})
		assertNoErr(t, err)
		assertNotEmpty(t, diskPath, "expected diskPath")

		resp, ok, err := s.Get(ctx, "empty")
		assertNoErr(t, err)
		assertTrue(t, ok, "expected hit")
		assertEqual(t, int64(0), resp.BodySize)
	})

	t.Run("put is atomic: partial failure does not corrupt existing entry", func(t *testing.T) {
		s := NewFileSystemStorage(t.TempDir())
		// first put — good data
		const original = "original body"
		_, err := s.Put(ctx, PutRequest{
			Key:      "key1",
			OutputID: []byte("out1"),
			Body:     strings.NewReader(original),
			BodySize: int64(len(original)),
		})
		assertNoErr(t, err)

		// second put — error during read (reader immediately returns error)
		_, err = s.Put(ctx, PutRequest{
			Key:      "key1",
			OutputID: []byte("out2"),
			Body:     &errorReader{err: io.ErrUnexpectedEOF},
			BodySize: 999,
		})
		assertErr(t, err)

		// original data must still be readable
		resp, ok, err := s.Get(ctx, "key1")
		assertNoErr(t, err)
		assertTrue(t, ok, "expected original entry still present")
		assertEqual(t, "out1", string(resp.OutputID))
	})

	t.Run("put with empty key returns error", func(t *testing.T) {
		s := NewFileSystemStorage(t.TempDir())
		_, err := s.Put(ctx, PutRequest{
			Key:  "",
			Body: strings.NewReader("x"),
		})
		assertErr(t, err)
	})

	t.Run("large body round-trip", func(t *testing.T) {
		s := NewFileSystemStorage(t.TempDir())
		body := must(randomString(1 << 20)) // 1 MB
		diskPath, err := s.Put(ctx, PutRequest{
			Key:      "bigkey",
			OutputID: []byte("bigout"),
			Body:     strings.NewReader(body),
			BodySize: int64(len(body)),
		})
		assertNoErr(t, err)
		data, err := os.ReadFile(diskPath)
		assertNoErr(t, err)
		assertEqual(t, body, string(data))
	})

	t.Run("overwrites existing key", func(t *testing.T) {
		s := NewFileSystemStorage(t.TempDir())
		put := func(body, key string) {
			t.Helper()
			_, err := s.Put(ctx, PutRequest{
				Key:      "k",
				OutputID: []byte(key),
				Body:     strings.NewReader(body),
				BodySize: int64(len(body)),
			})
			assertNoErr(t, err)
		}
		put("first", "out1")
		put("second", "out2")

		resp, ok, err := s.Get(ctx, "k")
		assertNoErr(t, err)
		assertTrue(t, ok, "expected hit")
		assertEqual(t, "out2", string(resp.OutputID))
		data, err := os.ReadFile(resp.DiskPath)
		assertNoErr(t, err)
		assertEqual(t, "second", string(data))
	})

	t.Run("close is a no-op", func(t *testing.T) {
		s := NewFileSystemStorage(t.TempDir())
		assertNoErr(t, s.Close(ctx))
	})
}

// errorReader always returns the given error.
type errorReader struct{ err error }

func (e *errorReader) Read([]byte) (int, error) { return 0, e.err }
