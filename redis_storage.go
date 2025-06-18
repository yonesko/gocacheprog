package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"io"
	"strings"
	"time"
)

type (
	redisStorage struct {
		cluster *redis.ClusterClient
	}
	meta struct {
		OutputID []byte
		Size     int64
	}
)

func NewRedisStorage(cluster *redis.ClusterClient) Storage {
	return &redisStorage{cluster: cluster}
}

func (r redisStorage) Get(ctx context.Context, key string) (GetResponse, bool, error) {
	body, m, ok, err := r.get(ctx, key)
	if err != nil {
		return GetResponse{}, false, err
	}
	if !ok {
		return GetResponse{}, false, nil
	}

	return GetResponse{
		OutputID: m.OutputID,
		DiskPath: "", //DiskPath return only from FS-storage
		BodySize: m.Size,
		Body:     body,
	}, true, nil
}

func (r redisStorage) get(ctx context.Context, key string) (io.Reader, meta, bool, error) {
	if strings.TrimSpace(key) == "" {
		return nil, meta{}, false, fmt.Errorf("empty key")
	}
	keyBody, keyMeta := r.keyNames(key)
	bodyGet := r.cluster.Get(ctx, keyBody)
	err := bodyGet.Err()
	if errors.Is(err, redis.Nil) {
		return nil, meta{}, false, nil
	}
	if err != nil {
		return nil, meta{}, false, fmt.Errorf("redis bodyGet error: %w", err)
	}
	metaGet := r.cluster.Get(ctx, keyMeta)
	err = metaGet.Err()
	if errors.Is(err, redis.Nil) {
		return nil, meta{}, false, nil
	}
	if err != nil {
		return nil, meta{}, false, fmt.Errorf("redis metaGet error: %w", err)
	}
	var m meta
	err = json.Unmarshal(must(metaGet.Bytes()), &m)
	if err != nil {
		return nil, meta{}, false, fmt.Errorf("redis metaGet Unmarshal error: %w", err)
	}
	b, err := bodyGet.Bytes()
	if err != nil {
		return nil, meta{}, false, fmt.Errorf("redis bodyGet Bytes error: %w", err)
	}
	return bytes.NewReader(b), m, true, nil
}

func (r redisStorage) Put(ctx context.Context, request PutRequest) (string, error) {
	const expiration = time.Hour * 24 * 7
	keyBody, keyMeta := r.keyNames(request.Key)
	b, err := io.ReadAll(request.Body)
	if err != nil {
		return "", fmt.Errorf("redis bodyReadAll error: %w", err)
	}
	set := r.cluster.Set(ctx, keyBody, b, expiration)
	err = set.Err()
	if err != nil {
		return "", fmt.Errorf("redis set error: %w", err)
	}
	set = r.cluster.Set(ctx, keyMeta, must(json.Marshal(meta{OutputID: request.OutputID, Size: request.BodySize})), expiration)
	err = set.Err()
	if err != nil {
		return "", fmt.Errorf("redis set error: %w", err)
	}
	//no disk path to return
	return "", nil
}

func (r redisStorage) Close(_ context.Context) error {
	return r.cluster.Close()
}

func (r redisStorage) keyNames(key string) (keyBody, keyMeta string) {
	key = "gocacheprog/" + key
	keyBody = key + "-o"
	keyMeta = key + "-i"
	return keyBody, keyMeta
}
