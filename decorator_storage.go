package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
)

// use one storage to save to disk and one to the external storage
type decoratorStorage struct {
	diskStorage     Storage
	externalStorage Storage
}

func NewDecoratorStorage(diskStorage Storage, externalStorage Storage) Storage {
	return &decoratorStorage{diskStorage: diskStorage, externalStorage: externalStorage}
}

func (s decoratorStorage) Get(ctx context.Context, key string) (GetResponse, bool, error) {
	getResponse, ok, err := s.diskStorage.Get(ctx, key)
	if err != nil {
		return GetResponse{}, false, fmt.Errorf("unable to get key %s: %w", key, err)
	}
	if ok {
		return getResponse, true, nil
	}
	//download to disk and return
	getResponse, ok, err = s.externalStorage.Get(ctx, key)
	if err != nil {
		return GetResponse{}, false, fmt.Errorf("could not get key from %s: %w", key, err)
	}
	if !ok {
		return GetResponse{}, false, nil
	}
	if getResponse.Body == nil {
		return GetResponse{}, false, fmt.Errorf("empty getResponse.Body %s", key)
	}
	_, err = s.diskStorage.Put(ctx, PutRequest{
		Key:      key,
		OutputID: getResponse.OutputID,
		Body:     getResponse.Body,
		BodySize: getResponse.BodySize,
	})
	if err != nil {
		return GetResponse{}, false, fmt.Errorf("failed to store response: %w", err)
	}
	return s.diskStorage.Get(ctx, key)
}

func (s decoratorStorage) Put(ctx context.Context, request PutRequest) (string, error) {
	//TODO use Tee
	buffer := bytes.NewReader(must(io.ReadAll(request.Body)))
	request.Body = buffer
	diskPath, err := s.diskStorage.Put(ctx, request)
	if err != nil {
		return "", fmt.Errorf("could not store response: %w", err)
	}
	//TODO make concurrent
	//buffer.
	io.Copy(os.Stdout, buffer)
	buffer.Seek(0, io.SeekStart)
	s.externalStorage.Put(ctx, PutRequest{
		Key:      request.Key,
		OutputID: request.OutputID,
		Body:     buffer,
		BodySize: request.BodySize,
	})
	return diskPath, nil
}

func (s decoratorStorage) Close(ctx context.Context) error {
	err1 := s.diskStorage.Close(ctx)
	err2 := s.externalStorage.Close(ctx)
	if err1 == nil && err2 == nil {
		return nil
	}
	return fmt.Errorf("%w %w", err1, err2)
}
