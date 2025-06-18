package main

import (
	"context"
	"github.com/redis/go-redis/v9"
)

type redisStorage struct {
	cluster *redis.ClusterClient
}

func (r redisStorage) Get(ctx context.Context, key string) (GetResponse, bool, error) {
	//TODO implement me
	panic("implement me")
}

func (r redisStorage) Put(ctx context.Context, request PutRequest) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (r redisStorage) Close(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}
