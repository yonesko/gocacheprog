package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"github.com/redis/go-redis/v9"
	"io"
	"log"
	"os"
)

var (
	logResponse            = flag.Bool("log_resp", false, "log responses")
	logRequest             = flag.Bool("log_req", false, "log requests")
	dir                    = flag.String("dir", "", "dir of cache")
	inputReader  io.Reader = os.Stdin
	outputWriter io.Writer = os.Stdout
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

func connectCluster() *redis.ClusterClient {
	rdb := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs: []string{
			"10.0.4.153:7000",
			"10.0.4.154:7000",
			"10.0.4.155:7000",
		},
	})

	// Ping to confirm connection
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("Could not connect to Redis Cluster: %v", err)
	}

	return rdb
}

func main() {
	connectCluster()
	flag.Parse()
	if *dir == "" {
		flag.Usage()
		log.Fatal("dir is required")
	}
	if *logResponse {
		outputWriter = newLoggingWriter(os.Stdout)
	}
	if *logRequest {
		inputReader = newLoggingReader(os.Stdin)
	}
	//handshake
	resp(Response{KnownCommands: []Cmd{CmdGet, CmdPut, CmdClose}}, nil)
	//
	ctx := context.Background()
	storage := NewStat(NewFileSystemStorage(*dir))
	keyConverter := hex.EncodeToString
	reader := json.NewDecoder(bufio.NewReader(inputReader))
	for {
		var request Request
		if err := reader.Decode(&request); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			panic(err)
		}
		if request.Command == CmdPut {
			if request.BodySize > 0 {
				//TODO stream
				var body []byte
				must0(reader.Decode(&body))
				request.Body = bytes.NewReader(body)
			} else {
				request.Body = bytes.NewBuffer(nil)
			}
			diskPath, err := storage.Put(ctx, PutRequest{
				Key:      keyConverter(request.ActionID),
				OutputID: request.OutputID,
				Body:     request.Body,
				BodySize: request.BodySize,
			})
			resp(Response{ID: request.ID, DiskPath: diskPath}, err)
			continue
		}

		if request.Command == CmdGet {
			entry, ok, err := storage.Get(ctx, keyConverter(request.ActionID))
			resp(Response{
				ID:       request.ID,
				Miss:     !ok,
				DiskPath: entry.DiskPath,
				OutputID: entry.OutputID,
				Size:     entry.BodySize,
			}, err)
			continue
		}
		if request.Command == CmdClose {
			err := storage.Close(ctx)
			resp(Response{ID: request.ID}, err)
			os.Exit(0)
		}
	}
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

func resp(response Response, err error) {
	if err != nil {
		response.Err = err.Error()
	}
	b := must(json.Marshal(response))
	must(outputWriter.Write(b))
	must(outputWriter.Write([]byte{'\n'}))
}
