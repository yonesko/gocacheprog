package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
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
	_key := r.keyNames(key)
	res, err := r.cluster.HMGet(ctx, _key, "body", "meta").Result()
	if err != nil {
		return nil, meta{}, false, fmt.Errorf("redis hmget error: %w %s", err, _key)
	}

	if len(res) < 2 || res[0] == nil || res[1] == nil {
		return nil, meta{}, false, nil
	}

	bodyStr, ok1 := res[0].(string)
	metaStr, ok2 := res[1].(string)
	if !ok1 || !ok2 {
		return nil, meta{}, false, fmt.Errorf("redis data type assertion error for key: %s", _key)
	}

	var m meta
	err = json.Unmarshal([]byte(metaStr), &m)
	if err != nil {
		return nil, meta{}, false, fmt.Errorf("redis meta Unmarshal error: %w %s", err, _key)
	}

	// Возвращаем ридер на основе байт тела
	return bytes.NewReader([]byte(bodyStr)), m, true, nil
}

func (r redisStorage) Put(ctx context.Context, request PutRequest) (string, error) {
	const expiration = time.Hour * 24 * 7
	key := r.keyNames(request.Key)
	b, err := io.ReadAll(request.Body)
	if err != nil {
		return "", fmt.Errorf("redis bodyReadAll error: %w %s", err, request.Key)
	}

	metaBytes, err := json.Marshal(meta{OutputID: request.OutputID, Size: request.BodySize})
	if err != nil {
		return "", fmt.Errorf("redis metaMarshal error: %w %s", err, request.Key)
	}

	pipe := r.cluster.Pipeline()
	pipe.HSet(ctx, key, map[string]any{
		"body": b,
		"meta": metaBytes,
	})
	pipe.Expire(ctx, key, expiration)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return "", fmt.Errorf("redis hset error: %w %s", err, key)
	}

	return "", nil
}
func (r redisStorage) Close(_ context.Context) error {
	return r.cluster.Close()
}

func (r redisStorage) keyNames(key string) string {
	parts := []string{"gocacheprog"}
	if r.redisKeyPrefix != "" {
		parts = append(parts, r.redisKeyPrefix)
	}
	parts = append(parts, key)
	key = path.Join(parts...)
	return key
}
