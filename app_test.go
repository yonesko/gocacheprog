package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestApp_Run(t *testing.T) {
	buffer := &bytes.Buffer{}
	app := NewApp(
		marshalCmds(
			Request{ID: 1, Command: CmdPut, ActionID: []byte("ActionID_1"), OutputID: []byte("OutputID_1"), BodySize: 5, Body: strings.NewReader(must(randomString(5)))},
			Request{ID: 2, Command: CmdPut, ActionID: []byte("ActionID_2"), OutputID: []byte("OutputID_2"), BodySize: 6, Body: strings.NewReader(must(randomString(6)))},
		),
		buffer,
		hex.EncodeToString,
		NewFileSystemStorage(t.TempDir()),
	)
	app.Run(context.Background())
	fmt.Println(buffer.String())
}

/*
goos: darwin
goarch: arm64
pkg: gocacheprog
cpu: Apple M1 Pro
Benchmark/many_gets-10         	     864	   1289663 ns/op
Benchmark/many_puts-10         	     674	   1656220 ns/op
*/
func Benchmark(b *testing.B) {
	b.Run("many gets", func(b *testing.B) {
		s := buildGets(1000)
		for i := 0; i < b.N; i++ {
			app := NewApp(
				strings.NewReader(s),
				io.Discard,
				hex.EncodeToString,
				sleepingStorage{},
			)
			app.Run(context.Background())
		}
	})
	b.Run("many puts", func(b *testing.B) {
		s := buildPuts(1000)
		for i := 0; i < b.N; i++ {
			app := NewApp(
				strings.NewReader(s),
				io.Discard,
				hex.EncodeToString,
				sleepingStorage{},
			)
			app.Run(context.Background())
		}
	})
}

func buildGets(n int) string {
	var requests []Request
	for i := range n {
		requests = append(requests, Request{
			ID:       int64(i),
			Command:  CmdGet,
			ActionID: []byte(strconv.Itoa(i)),
			OutputID: []byte(strconv.Itoa(i)),
		})
	}
	return string(must(io.ReadAll(marshalCmds(requests...))))
}

func buildPuts(n int) string {
	var requests []Request
	for i := range n {
		requests = append(requests, Request{
			ID:       int64(i),
			Command:  CmdPut,
			ActionID: []byte("ActionID_" + strconv.Itoa(i)),
			OutputID: []byte("OutputID_1" + strconv.Itoa(i)),
			BodySize: 5,
			Body:     strings.NewReader(must(randomString(5))),
		})
	}
	return string(must(io.ReadAll(marshalCmds(requests...))))
}
func marshalCmds(requests ...Request) io.Reader {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	for _, r := range requests {
		must0(encoder.Encode(r))
		if r.Command == CmdPut {
			must0(encoder.Encode(must(io.ReadAll(r.Body))))
		}
	}
	return buffer
}

type sleepingStorage struct {
}

func (t sleepingStorage) Get(ctx context.Context, key string) (GetResponse, bool, error) {
	time.Sleep(time.Microsecond)
	return GetResponse{}, true, nil
}

func (t sleepingStorage) Put(ctx context.Context, request PutRequest) (string, error) {
	time.Sleep(time.Microsecond)
	return "", nil
}

func (t sleepingStorage) Close(ctx context.Context) error {
	time.Sleep(time.Microsecond)
	return nil
}

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func randomString(length int) (string, error) {
	result := make([]byte, length)
	for i := range result {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[num.Int64()]
	}
	return string(result), nil
}
