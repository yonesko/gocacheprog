package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

var (
	debug                  = flag.Bool("v", false, "enable verbose output")
	dir                    = flag.String("dir", "", "dir of cache")
	inputReader  io.Reader = newLoggingReader(os.Stdin)
	outputWriter io.Writer = newLoggingWriter(os.Stdout)
	outputCh               = make(chan []byte)
)

type (
	GetResponse struct {
		OutputID []byte
		DiskPath string
	}
	PutRequest struct {
		Key      string
		OutputID []byte
		Body     io.Reader
		BodySize int64
	}
	Storage interface {
		Get(ctx context.Context, key string) (GetResponse, bool, error)
		//Put returns DiskPath TODO add size
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
	waitGroup := sync.WaitGroup{}
	waitGroup.Add(1)
	go func() {
		defer waitGroup.Done()
		writer := bufio.NewWriter(outputWriter)
		defer writer.Flush()
		ticker := time.NewTicker(time.Millisecond)
		for {
			select {
			case b, ok := <-outputCh:
				if !ok {
					return
				}
				must(writer.Write(b))
			case <-ticker.C:
				if writer.Buffered() > 0 {
					must0(writer.Flush())
				}
			}
		}
	}()
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
				//TODO checksum
				var body []byte
				must0(reader.Decode(&body))
				request.Body = bytes.NewReader(body)
			} else {
				request.Body = bytes.NewBuffer(nil)
			}
			go func(request Request) {
				diskPath, err := storage.Put(ctx, PutRequest{
					Key:      keyConverter(request.ActionID),
					OutputID: request.OutputID,
					Body:     request.Body,
					BodySize: request.BodySize,
				})
				resp(Response{ID: request.ID, DiskPath: diskPath}, err)
			}(request)
			continue
		}

		if request.Command == CmdGet {
			go func(request Request) {
				entry, ok, err := storage.Get(ctx, keyConverter(request.ActionID))
				resp(Response{ID: request.ID, Miss: ok, DiskPath: entry.DiskPath, OutputID: entry.OutputID}, err)
			}(request)
			continue
		}
		if request.Command == CmdClose {
			err := storage.Close(ctx)
			resp(Response{ID: request.ID}, err)
			close(outputCh)
			waitGroup.Wait()
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
	outputCh <- b
}
