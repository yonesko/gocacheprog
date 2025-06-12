package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
)

var (
	debug = flag.Bool("v", false, "enable verbose output")
	dir   = flag.String("dir", "", "dir of cache")
	//tempDir = os.TempDir()
)

type (
	Entry struct {
		OutputID []byte
		DiskPath string
	}
	Storage interface {
		Get(ctx context.Context, key string) (Entry, bool, error)
		//Put returns DiskPath TODO add size
		Put(ctx context.Context, key string, outputID []byte, body io.Reader) (string, error)
		Close(ctx context.Context) error
	}
)

func main() {
	flag.Parse()
	if *dir == "" {
		flag.Usage()
		log.Fatal("dir is required")
	}
	//handshake
	resp(Response{KnownCommands: []Cmd{CmdGet, CmdPut, CmdClose}}, nil)
	//
	ctx := context.Background()
	storage := NewStat(NewFileSystemStorage(*dir))
	keyConverter := hex.EncodeToString
	reader := json.NewDecoder(os.Stdin)
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
			diskPath, err := storage.Put(ctx, keyConverter(request.ActionID), request.OutputID, request.Body)
			resp(Response{ID: request.ID, DiskPath: diskPath}, err)
			continue
		}

		if request.Command == CmdGet {
			entry, ok, err := storage.Get(ctx, keyConverter(request.ActionID))
			resp(Response{ID: request.ID, Miss: ok, DiskPath: entry.DiskPath, OutputID: entry.OutputID}, err)
			continue
		}
		if request.Command == CmdClose {
			err := storage.Close(ctx)
			resp(Response{ID: request.ID}, err)
			must0(os.Stdout.Close())
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
	_, _ = os.Stdout.Write(b)
	if *debug {
		os.Stderr.WriteString(fmt.Sprintf("< %s\n", string(b)))
	}

}
