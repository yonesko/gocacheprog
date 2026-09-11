package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"
)

func Test_DecoratorStorage(t *testing.T) {
	ctx := context.Background()

	t.Run("get miss in both storages", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ext := NewMockStorage(ctrl)
		ext.EXPECT().Get(gomock.Any(), "key1").Return(GetResponse{}, false, nil).Times(1)
		ext.EXPECT().Close(gomock.Any()).Return(nil).Times(1)

		s := NewDecoratorStorage(NewFileSystemStorage(t.TempDir()), ext)
		_, ok, err := s.Get(ctx, "key1")
		assertNoErr(t, err)
		assertFalse(t, ok, "expected miss")
		assertNoErr(t, s.Close(ctx))
	})

	t.Run("get hit from filesystem, external not called", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ext := NewMockStorage(ctrl)
		// external must NOT be called if FS already has the entry
		ext.EXPECT().Get(gomock.Any(), gomock.Any()).Times(0)
		ext.EXPECT().Close(gomock.Any()).Return(nil).Times(1)

		fsDir := t.TempDir()
		fs := NewFileSystemStorage(fsDir)
		// seed the FS directly
		_, err := fs.Put(ctx, PutRequest{
			Key:      "key1",
			OutputID: []byte("out1"),
			Body:     strings.NewReader("body"),
			BodySize: 4,
		})
		assertNoErr(t, err)

		s := NewDecoratorStorage(fs, ext)
		resp, ok, err := s.Get(ctx, "key1")
		assertNoErr(t, err)
		assertTrue(t, ok, "expected hit")
		assertEqual(t, "out1", string(resp.OutputID))
		assertNoErr(t, s.Close(ctx))
	})

	t.Run("get miss in FS, hit in external: downloads to FS", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ext := NewMockStorage(ctrl)
		ext.EXPECT().Get(gomock.Any(), "key1").
			Return(GetResponse{
				OutputID: []byte("out1"),
				Body:     strings.NewReader("hello"),
				BodySize: 5,
			}, true, nil).Times(1)
		ext.EXPECT().Close(gomock.Any()).Return(nil).Times(1)

		fsDir := t.TempDir()
		s := NewDecoratorStorage(NewFileSystemStorage(fsDir), ext)

		resp, ok, err := s.Get(ctx, "key1")
		assertNoErr(t, err)
		assertTrue(t, ok, "expected hit")
		assertEqual(t, "out1", string(resp.OutputID))
		// DiskPath must be set — file was written to FS
		assertNotEmpty(t, resp.DiskPath, "expected DiskPath to be set after download")

		// second Get must hit FS only (external not called again — Times(1) above)
		resp2, ok2, err2 := s.Get(ctx, "key1")
		assertNoErr(t, err2)
		assertTrue(t, ok2, "expected hit on second get")
		assertEqual(t, "out1", string(resp2.OutputID))

		assertNoErr(t, s.Close(ctx))
	})

	t.Run("get error from external propagates", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ext := NewMockStorage(ctrl)
		ext.EXPECT().Get(gomock.Any(), "key1").
			Return(GetResponse{}, false, fmt.Errorf("redis down")).Times(1)
		ext.EXPECT().Close(gomock.Any()).Return(nil).Times(1)

		s := NewDecoratorStorage(NewFileSystemStorage(t.TempDir()), ext)
		_, ok, err := s.Get(ctx, "key1")
		assertErr(t, err)
		assertFalse(t, ok, "expected miss on error")
		assertNoErr(t, s.Close(ctx))
	})

	t.Run("put writes to both FS and external", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ext := NewMockStorage(ctrl)
		var extBody []byte
		ext.EXPECT().Put(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, req PutRequest) (string, error) {
				var err error
				extBody, err = io.ReadAll(req.Body)
				return "", err
			}).Times(1)
		ext.EXPECT().Close(gomock.Any()).Return(nil).Times(1)

		fsDir := t.TempDir()
		s := NewDecoratorStorage(NewFileSystemStorage(fsDir), ext)

		diskPath, err := s.Put(ctx, PutRequest{
			Key:      "key1",
			OutputID: []byte("out1"),
			Body:     strings.NewReader("hello world"),
			BodySize: 11,
		})
		assertNoErr(t, err)
		assertNotEmpty(t, diskPath, "expected diskPath from FS put")

		assertNoErr(t, s.Close(ctx)) // waits for background external Put

		assertEqual(t, "hello world", string(extBody))
	})

	t.Run("put returns disk path from FS even if external fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ext := NewMockStorage(ctrl)
		ext.EXPECT().Put(gomock.Any(), gomock.Any()).Return("", fmt.Errorf("redis timeout")).Times(1)
		ext.EXPECT().Close(gomock.Any()).Return(nil).Times(1)

		s := NewDecoratorStorage(NewFileSystemStorage(t.TempDir()), ext)
		diskPath, err := s.Put(ctx, PutRequest{
			Key:      "key1",
			OutputID: []byte("out1"),
			Body:     strings.NewReader("data"),
			BodySize: 4,
		})
		// external error is logged to stderr, not returned
		assertNoErr(t, err)
		assertNotEmpty(t, diskPath, "expected diskPath")
		assertNoErr(t, s.Close(ctx))
	})

	t.Run("close waits for all background puts to finish", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ext := NewMockStorage(ctrl)
		putDone := make(chan struct{})
		ext.EXPECT().Put(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, req PutRequest) (string, error) {
				io.ReadAll(req.Body)
				close(putDone)
				return "", nil
			}).Times(1)
		ext.EXPECT().Close(gomock.Any()).Return(nil).Times(1)

		s := NewDecoratorStorage(NewFileSystemStorage(t.TempDir()), ext)
		_, err := s.Put(ctx, PutRequest{
			Key:  "key1",
			Body: strings.NewReader("x"),
		})
		assertNoErr(t, err)

		assertNoErr(t, s.Close(ctx))
		select {
		case <-putDone:
			// good — background put completed before Close returned
		default:
			t.Fatal("Close returned before background external Put finished")
		}
	})

	t.Run("close returns both errors when both storages fail", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ext := NewMockStorage(ctrl)
		errExt := errors.New("ext close error")
		ext.EXPECT().Close(gomock.Any()).Return(errExt).Times(1)

		fsDir := t.TempDir()
		// use a failing FS mock instead
		failFS := NewMockStorage(ctrl)
		errFS := errors.New("fs close error")
		failFS.EXPECT().Get(gomock.Any(), gomock.Any()).Return(GetResponse{}, false, nil).AnyTimes()
		failFS.EXPECT().Put(gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()
		failFS.EXPECT().Close(gomock.Any()).Return(errFS).Times(1)
		_ = fsDir

		s := NewDecoratorStorage(failFS, ext)
		err := s.Close(ctx)
		assertErr(t, err)
		assertTrue(t, errors.Is(err, errFS), "expected fs error to be wrapped")
		assertTrue(t, errors.Is(err, errExt), "expected ext error to be wrapped")
	})
}

func Benchmark_DecoratorStorage(b *testing.B) {
	b.Run("put", func(b *testing.B) {
		ctrl := gomock.NewController(b)
		ext := NewMockStorage(ctrl)
		ext.EXPECT().Put(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req PutRequest) (string, error) {
			io.ReadAll(req.Body)
			return "", nil
		}).AnyTimes()
		ext.EXPECT().Close(gomock.Any()).Return(nil).AnyTimes()

		s := NewDecoratorStorage(NewFileSystemStorage(b.TempDir()), ext)
		body := must(randomString(100))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := s.Put(context.Background(), PutRequest{
				Key:      "dcd3McUV",
				OutputID: []byte("out"),
				Body:     strings.NewReader(body),
				BodySize: int64(len(body)),
			})
			if err != nil {
				b.Fatal(err)
			}
		}
		s.Close(context.Background())
	})
}
