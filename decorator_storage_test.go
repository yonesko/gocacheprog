package main

import (
	"context"
	"fmt"
	"go.uber.org/mock/gomock"
	"strings"
	"testing"
)

func Test_DecoratorStorage(t *testing.T) {
	t.Run("get miss in both storages", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		externalStorage := NewMockStorage(ctrl)
		externalStorage.EXPECT().Get(gomock.Any(), "fOwaAFKWb").Return(GetResponse{}, false, nil).Times(1)
		externalStorage.EXPECT().Close(gomock.Any()).Return(nil).Times(1)

		storage := NewDecoratorStorage(
			NewFileSystemStorage(t.TempDir()),
			externalStorage,
		)
		get, ok, err := storage.Get(context.Background(), "fOwaAFKWb")
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Fatal("expected to be missing")
		}
		if get.OutputID != nil {
			t.Fatal("expected to be missing")
		}
		err = storage.Close(context.Background())
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("get miss only in FS storage", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		externalStorage := NewMockStorage(ctrl)
		externalStorage.EXPECT().Get(gomock.Any(), "fOwaAFKWb").
			Return(GetResponse{OutputID: []byte("MinRana"), Body: strings.NewReader("")}, true, nil).Times(1)
		externalStorage.EXPECT().Close(gomock.Any()).Return(nil).Times(1)

		storage := NewDecoratorStorage(NewFileSystemStorage(t.TempDir()), externalStorage)
		get, ok, err := storage.Get(context.Background(), "fOwaAFKWb")
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatal("expected to be found")
		}
		if string(get.OutputID) != "MinRana" {
			t.Fatal("expected to be equal")
		}
		err = storage.Close(context.Background())
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("external storage return err - return err too", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		externalStorage := NewMockStorage(ctrl)
		externalStorage.EXPECT().Get(gomock.Any(), "fOwaAFKWb").
			Return(GetResponse{}, false, fmt.Errorf("LihuaJones")).Times(1)
		externalStorage.EXPECT().Close(gomock.Any()).Return(nil).Times(1)

		storage := NewDecoratorStorage(NewFileSystemStorage(t.TempDir()), externalStorage)
		_, ok, err := storage.Get(context.Background(), "fOwaAFKWb")
		if err == nil {
			t.Fatal("expected to be err")
		}
		if ok {
			t.Fatal("expected to be missing")
		}
		err = storage.Close(context.Background())
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("get returns after put", func(t *testing.T) {
		const key = "fOwaAFKWb"
		ctrl := gomock.NewController(t)
		externalStorage := NewMockStorage(ctrl)
		externalStorage.EXPECT().Put(gomock.Any(), gomock.Any()).
			Return("", nil).Times(1)
		externalStorage.EXPECT().Close(gomock.Any()).Return(nil).Times(1)

		storage := NewDecoratorStorage(NewFileSystemStorage(t.TempDir()), externalStorage)
		diskPath, err := storage.Put(context.Background(), PutRequest{
			Key:      key,
			OutputID: []byte("MinRana"),
			Body:     strings.NewReader(must(randomString(100))),
			BodySize: 100,
		})
		if err != nil {
			t.Fatal(err)
		}
		if diskPath == "" {
			t.Fatal("expected disk path)")
		}

		_, ok, err := storage.Get(context.Background(), key)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatal("expected to be found")
		}

		err = storage.Close(context.Background())
		if err != nil {
			t.Fatal(err)
		}
	})
}

func Benchmark_DecoratorStorage(b *testing.B) {
	b.Run("put", func(b *testing.B) {
		storage := NewDecoratorStorage(NewFileSystemStorage(b.TempDir()), sleepingStorage{})
		request := PutRequest{
			Key:      "dcd3McUV",
			OutputID: []byte("MinRana"),
			Body:     strings.NewReader(must(randomString(100))),
			BodySize: 100,
		}
		for i := 0; i < b.N; i++ {
			_, err := storage.Put(context.Background(), request)
			if err != nil {
				b.Fatalf("expected no error, got %v", err)
			}
		}
	})
}
