package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"io"
	"path"
	"strings"
	"time"
)

type (
	redisStorage struct {
		cluster        redis.UniversalClient
		redisKeyPrefix string
	}
	meta struct {
		OutputID []byte
		Size     int64
	}
)

func NewRedisStorage(cluster redis.UniversalClient, redisKeyPrefix string) Storage {
	return &redisStorage{cluster: cluster, redisKeyPrefix: strings.TrimSpace(redisKeyPrefix)}
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
		return nil, meta{}, false, fmt.Errorf("redis bodyGet error: %w %s", err, key)
	}
	metaGet := r.cluster.Get(ctx, keyMeta)
	err = metaGet.Err()
	if errors.Is(err, redis.Nil) {
		return nil, meta{}, false, nil
	}
	if err != nil {
		return nil, meta{}, false, fmt.Errorf("redis metaGet error: %w %s", err, key)
	}
	var m meta
	metaBytes, err := metaGet.Bytes()
	if err != nil {
		return nil, meta{}, false, fmt.Errorf("redis metaGet error: %w %s", err, key)
	}
	err = json.Unmarshal(metaBytes, &m)
	if err != nil {
		return nil, meta{}, false, fmt.Errorf("redis metaGet Unmarshal error: %w %s", err, key)
	}
	b, err := bodyGet.Bytes()
	if err != nil {
		return nil, meta{}, false, fmt.Errorf("redis bodyGet Bytes error: %w %s", err, key)
	}
	return bytes.NewReader(b), m, true, nil
}

func (r redisStorage) Put(ctx context.Context, request PutRequest) (string, error) {
	const expiration = time.Hour * 24 * 7
	keyBody, keyMeta := r.keyNames(request.Key)
	b, err := io.ReadAll(request.Body)
	if err != nil {
		return "", fmt.Errorf("redis bodyReadAll error: %w %s", err, request.Key)
	}
	set := r.cluster.Set(ctx, keyBody, b, expiration)
	err = set.Err()
	if err != nil {
		return "", fmt.Errorf("redis set error: %w %s", err, request.Key)
	}
	metaBytes, err := json.Marshal(meta{OutputID: request.OutputID, Size: request.BodySize})
	if err != nil {
		return "", fmt.Errorf("redis metaMarshal error: %w %s", err, request.Key)
	}
	set = r.cluster.Set(ctx, keyMeta, metaBytes, expiration)
	err = set.Err()
	if err != nil {
		return "", fmt.Errorf("redis set error: %w %s", err, request.Key)
	}
	//no disk path to return
	return "", nil
}

func (r redisStorage) Close(_ context.Context) error {
	return r.cluster.Close()
}

func (r redisStorage) keyNames(key string) (keyBody, keyMeta string) {
	parts := []string{"gocacheprog"}
	if r.redisKeyPrefix != "" {
		parts = append(parts, r.redisKeyPrefix)
	}
	parts = append(parts, key)
	key = path.Join(parts...)
	keyBody = key + "-o"
	keyMeta = key + "-i"
	return keyBody, keyMeta
}
