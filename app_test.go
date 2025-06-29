package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestApp_Run(t *testing.T) {
	t.Run("parallel get and put of the same file", func(t *testing.T) {
		tempDir := t.TempDir()
		cmds := strings.Join(repeat(1000,
			string(marshalCmds(
				Request{Command: CmdGet, ActionID: []byte("ActionID_1"), OutputID: []byte("OutputID_1")},
				Request{Command: CmdPut, ActionID: []byte("ActionID_1"), OutputID: []byte("OutputID_1"), BodySize: 600, Body: strings.NewReader(must(randomString(600)))},
				Request{Command: CmdGet, ActionID: []byte("ActionID_1"), OutputID: []byte("OutputID_1")},
				Request{Command: CmdPut, ActionID: []byte("ActionID_1"), OutputID: []byte("OutputID_1"), BodySize: 666, Body: strings.NewReader(must(randomString(666)))},
			))),
			"\n")
		buffer := &safeBuffer{buf: bytes.Buffer{}}
		decoder := json.NewDecoder(buffer)
		app := NewApp(
			bytes.NewReader([]byte(cmds)),
			buffer,
			hex.EncodeToString,
			NewFileSystemStorage(tempDir),
		)
		app.Run(context.Background())
		responsesCount := 0
		for {
			var resp Response
			err := decoder.Decode(&resp)
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatal(fmt.Errorf("error decoding response: %w", err))
			}
			if resp.Err != "" {
				log.Fatal(resp.Err, tempDir)
			}
			responsesCount++
		}
		if responsesCount != 4000+1 {
			log.Fatal(fmt.Errorf("expected 4000 responses, got %d", responsesCount))
		}
	})
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
	return string(marshalCmds(requests...))
}

func buildPuts(n int) string {
	var requests []Request
	for i := range n {
		requests = append(requests, Request{
			ID:       int64(i),
			Command:  CmdPut,
			ActionID: []byte("ActionID_" + strconv.Itoa(i)),
			OutputID: []byte("OutputID_" + strconv.Itoa(i)),
			BodySize: 5,
			Body:     strings.NewReader(must(randomString(5))),
		})
	}
	return string(marshalCmds(requests...))
}
func marshalCmds(requests ...Request) []byte {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	for _, r := range requests {
		must0(encoder.Encode(r))
		if r.Command == CmdPut {
			must0(encoder.Encode(must(io.ReadAll(r.Body))))
		}
	}
	return buffer.Bytes()
}

type sleepingStorage struct {
}

func (t sleepingStorage) Get(ctx context.Context, key string) (GetResponse, bool, error) {
	time.Sleep(time.Millisecond)
	return GetResponse{}, true, nil
}

func (t sleepingStorage) Put(ctx context.Context, request PutRequest) (string, error) {
	time.Sleep(time.Millisecond)
	return "", nil
}

func (t sleepingStorage) Close(ctx context.Context) error {
	time.Sleep(time.Millisecond)
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

func repeat(count int, initial string) []string {
	result := make([]string, 0, count)

	for i := 0; i < count; i++ {
		result = append(result, initial)
	}

	return result
}

type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *safeBuffer) Read(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Read(p)
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}
