package main

import (
	"context"
	"fmt"
	"os"
)

type logStorage struct {
	Storage
}

func NewLogStorage(storage Storage) Storage {
	return &logStorage{Storage: storage}
}

func (l logStorage) Get(ctx context.Context, key string) (GetResponse, bool, error) {
	get, b, err := l.Storage.Get(ctx, key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "get error: %v\n", err)
	}
	return get, b, err
}

func (l logStorage) Put(ctx context.Context, request PutRequest) (string, error) {
	put, err := l.Storage.Put(ctx, request)
	if err != nil {
		fmt.Fprintf(os.Stderr, "put error: %v\n", err)
	}
	return put, err
}

func (l logStorage) Close(ctx context.Context) error {
	err := l.Storage.Close(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "close error: %v\n", err)
	}
	return err
}
