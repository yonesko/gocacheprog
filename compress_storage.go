package main

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
)

type compressStorage struct {
	Storage
}

func NewCompressStorage(storage Storage) Storage {
	return &compressStorage{Storage: storage}
}

func (c compressStorage) Get(ctx context.Context, key string) (GetResponse, bool, error) {
	getResponse, ok, err := c.Storage.Get(ctx, key)
	if err != nil {
		return GetResponse{}, false, err
	}
	buffer := &bytes.Buffer{}
	encoder, err := zstd.NewReader(buffer)
	if err != nil {
		return GetResponse{}, false, fmt.Errorf("get: zstd compressor: %w", err)
	}
	_, err = io.Copy(buffer, encoder)
	if err != nil {
		return GetResponse{}, false, fmt.Errorf("get: zstd decompressor: %w", err)
	}
	return getResponse, ok, err
}

func (c compressStorage) Put(ctx context.Context, request PutRequest) (string, error) {
	buffer := &bytes.Buffer{}
	encoder, err := zstd.NewWriter(buffer)
	if err != nil {
		return "", fmt.Errorf("put: zstd compressor: %w", err)
	}
	defer encoder.Close()
	_, err = io.Copy(encoder, request.Body)
	if err != nil {
		return "", fmt.Errorf("put: zstd compressor: %w", err)
	}
	return c.Storage.Put(ctx, PutRequest{
		Key:      request.Key,
		OutputID: request.OutputID,
		Body:     buffer,
		BodySize: request.BodySize,
	})
}

func (c compressStorage) Close(ctx context.Context) error {
	return c.Storage.Close(ctx)
}
