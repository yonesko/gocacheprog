package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/redis/go-redis/v9"
	"io"
	"log"
	"os"
	"strings"
)

var (
	logResponse    = flag.Bool("log-resp", false, "log responses")
	logRequest     = flag.Bool("log-req", false, "log requests")
	logMetrics     = flag.Bool("log-metrics", false, "log metrics")
	dir            = flag.String("dir", "", "local dir of cache")
	redisUser      = flag.String("r-usr", "", "redis user")
	redisPassword  = flag.String("r-pwd", "", "redis password")
	redisAddresses = flag.String("r-urls", "", "comma separated redis addresses")
	redisKeyPrefix = flag.String("r-prefix", "", "string to prefix redis cache keys")
)

type (
	GetResponse struct {
		OutputID []byte
		DiskPath string
		BodySize int64
		Body     io.Reader
	}
	PutRequest struct {
		Key      string
		OutputID []byte
		Body     io.Reader
		BodySize int64
	}
	Storage interface {
		//Get asks for file, ensures that it exists at DiskPath, returns true if found
		Get(ctx context.Context, key string) (GetResponse, bool, error)
		//Put loads file, ensures that it exists at DiskPath, returns disk path
		Put(ctx context.Context, request PutRequest) (string, error)
		Close(ctx context.Context) error
	}
)

func main() {
	flag.Parse()
	if *dir == "" {
		flag.Usage()
		log.Fatal("dir is required")
	}
	var inputReader io.Reader = os.Stdin
	var outputWriter io.Writer = os.Stdout
	if *logResponse {
		outputWriter = newLoggingWriter(outputWriter)
	}
	if *logRequest {
		inputReader = newLoggingReader(inputReader)
	}
	ctx := context.Background()
	NewApp(inputReader, outputWriter, hex.EncodeToString, buildStorage()).
		Run(ctx)
}

func buildStorage() Storage {
	withMetrics := func(s Storage) Storage {
		if *logMetrics {
			return NewMetricsStorage(s)
		}
		return s
	}

	client, err := func() (redis.UniversalClient, error) {
		client := redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:      strings.Split(*redisAddresses, ","),
			ClientName: "gocacheprog",
			Username:   *redisUser,
			Password:   *redisPassword,
		})
		err := client.Ping(context.Background()).Err()
		if err != nil && err.Error() == "ERR This instance has cluster support disabled" {
			split := strings.Split(*redisAddresses, ",")
			if len(split) != 1 {
				return nil, fmt.Errorf("invalid redis address count: %s", *redisAddresses)
			}
			return redis.NewClient(&redis.Options{
				Addr:       split[0],
				ClientName: "gocacheprog",
				Username:   *redisUser,
				Password:   *redisPassword,
			}), nil
		}
		if err != nil {
			return nil, err
		}
		return client, nil
	}()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to redis server, switching to local file system: %s\n", err)
		return withMetrics(NewFileSystemStorage(*dir))
	}
	return NewLogStorage(withMetrics(NewDecoratorStorage(
		withMetrics(NewFileSystemStorage(*dir)),
		withMetrics(NewRedisStorage(
			client,
			*redisKeyPrefix,
		)),
	)))
}

func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}
	return t
}

func must0(err error) {
	if err != nil {
		panic(err)
	}
}
