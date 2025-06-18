package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"github.com/redis/go-redis/v9"
	"testing"
)

func TestNewRedisStorage(t *testing.T) {
	t.Skip("manual")
	addrs := []string{"10.0.4.153:7000",
		"10.0.4.154:7000",
		"10.0.4.155:7000",
		"10.0.4.153:7001",
		"10.0.4.154:7001",
		"10.0.4.155:7001",
	}
	storage := NewRedisStorage(redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:      addrs,
		ClientName: "gocacheprog",
		Username:   "gdanichev",
		Password:   "",
	}))

	const key = "bLqNioCWfF"
	get, ok, err := storage.Get(context.Background(), key)
	fmt.Println(get, ok, err)

	b := make([]byte, 16)
	rand.Read(b)
	body := make([]byte, 1024)
	rand.Read(body)
	put, err := storage.Put(context.Background(), PutRequest{
		Key:      key,
		OutputID: b,
		Body:     bytes.NewReader(body),
		BodySize: int64(len(body)),
	})
	fmt.Println(put, err)
}
