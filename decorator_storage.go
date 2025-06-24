package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
)

// use one storage to save to disk and one to the external storage
type decoratorStorage struct {
	fileSystemStorage Storage
	externalStorage   Storage
}

func NewDecoratorStorage(diskStorage Storage, externalStorage Storage) Storage {
	return &decoratorStorage{fileSystemStorage: diskStorage, externalStorage: externalStorage}
}

func (s decoratorStorage) Get(ctx context.Context, key string) (GetResponse, bool, error) {
	getResponse, ok, err := s.fileSystemStorage.Get(ctx, key)
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
	_, err = s.fileSystemStorage.Put(ctx, PutRequest{
		Key:      key,
		OutputID: getResponse.OutputID,
		Body:     getResponse.Body,
		BodySize: getResponse.BodySize,
	})
	if err != nil {
		return GetResponse{}, false, fmt.Errorf("failed to store response: %w", err)
	}
	return s.fileSystemStorage.Get(ctx, key)
}

func (s decoratorStorage) Put(ctx context.Context, request PutRequest) (string, error) {
	//TODO use Tee
	bodyBytes, err := io.ReadAll(request.Body)
	if err != nil {
		return "", fmt.Errorf("could not read body: %w", err)
	}
	request.Body = bytes.NewReader(bodyBytes)
	diskPath, err := s.fileSystemStorage.Put(ctx, request)
	if err != nil {
		return "", fmt.Errorf("could not store response: %w", err)
	}
	//TODO make concurrent
	_, err = s.externalStorage.Put(ctx, PutRequest{
		Key:      request.Key,
		OutputID: request.OutputID,
		Body:     bytes.NewReader(bodyBytes),
		BodySize: request.BodySize,
	})
	if err != nil {
		return "", fmt.Errorf("could not store external response: %w", err)
	}
	return diskPath, nil
}

func (s decoratorStorage) Close(ctx context.Context) error {
	err1 := s.fileSystemStorage.Close(ctx)
	err2 := s.externalStorage.Close(ctx)
	if err1 == nil && err2 == nil {
		return nil
	}
	return fmt.Errorf("%w %w", err1, err2)
}
