package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/klauspost/compress/zstd"
)

var (
	zstdDecoderPool = sync.Pool{
		New: func() any {
			reader, err := zstd.NewReader(nil)
			if err != nil {
				panic(err)
			}
			return reader
		},
	}
	zstdEncoderPool = sync.Pool{
		New: func() any {
			writer, err := zstd.NewWriter(nil)
			if err != nil {
				panic(err)
			}
			return writer
		},
	}
)

type compressStorage struct {
	Storage
}

func NewCompressStorage(storage Storage) Storage {
	return &compressStorage{Storage: storage}
}

func (c compressStorage) Get(ctx context.Context, key string) (GetResponse, bool, error) {
	getResponse, ok, err := c.Storage.Get(ctx, key)
	if err != nil || !ok {
		return getResponse, ok, err
	}
	decoder := zstdDecoderPool.Get().(*zstd.Decoder)
	err = decoder.Reset(getResponse.Body)
	if err != nil {
		return getResponse, false, fmt.Errorf("get: zstd decoder: %w", err)
	}
	buffer := &bytes.Buffer{}
	_, err = io.Copy(buffer, decoder)
	if err != nil {
		return GetResponse{}, false, fmt.Errorf("get: zstd decompress: %w", err)
	}
	getResponse.Body = buffer
	return getResponse, ok, nil
}

func (c compressStorage) Put(ctx context.Context, request PutRequest) (string, error) {
	buffer := &bytes.Buffer{}
	encoder := zstdEncoderPool.Get().(*zstd.Encoder)
	encoder.Reset(buffer)
	_, err := io.Copy(encoder, request.Body)
	if err != nil {
		return "", fmt.Errorf("put: zstd compressor: %w", err)
	}
	err = encoder.Close()
	if err != nil {
		return "", fmt.Errorf("put: zstd encoder: %w", err)
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
