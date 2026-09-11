package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

// Benchmark_App simulates a realistic incremental Go build workload:
//
//   - 200 build actions (medium-sized project)
//   - 70% cache hit rate (incremental rebuild after minor changes)
//   - body sizes approximating real Go package artifacts:
//     60% × 50 KB  (typical app package)
//     30% × 300 KB (medium stdlib package)
//     10% × 1 MB   (large stdlib package, e.g. net/http, crypto/tls)
//
// Request flow per action:
//
//	pre-seeded action: GET → hit  (no PUT)
//	fresh action:      GET → miss → PUT
func Benchmark_App(b *testing.B) {
	const (
		numActions   = 200
		hitRatio     = 0.70
		numPreSeeded = int(numActions * hitRatio) // 140 cached, 60 fresh
	)

	sizeOf := func(i int) int {
		switch {
		case i%10 == 0:
			return 1_000_000 // 10%: large package
		case i%10 < 4:
			return 300_000 // 30%: medium package
		default:
			return 50_000 // 60%: small package
		}
	}

	type action struct {
		actionID []byte
		outputID []byte
		body     []byte
	}

	// generate all action bodies once — incompressible random bytes
	actions := make([]action, numActions)
	totalBytes := int64(0)
	for i := range actions {
		size := sizeOf(i)
		body := make([]byte, size)
		rand.Read(body) //nolint
		actions[i] = action{
			actionID: []byte(fmt.Sprintf("action%04d", i)),
			outputID: []byte(fmt.Sprintf("output%04d", i)),
			body:     body,
		}
		totalBytes += int64(size)
	}

	// build the encoded request stream (base64 bodies included)
	// actions[0..numPreSeeded-1] → GET only (will hit pre-seeded cache)
	// actions[numPreSeeded..] → GET (miss) + PUT
	stream := func() string {
		buf := &bytes.Buffer{}
		enc := json.NewEncoder(buf)
		id := int64(1)
		for i, a := range actions {
			enc.Encode(Request{ //nolint
				ID:       id,
				Command:  CmdGet,
				ActionID: a.actionID,
			})
			id++
			if i >= numPreSeeded {
				enc.Encode(Request{ //nolint
					ID:       id,
					Command:  CmdPut,
					ActionID: a.actionID,
					OutputID: a.outputID,
					BodySize: int64(len(a.body)),
				})
				id++
				enc.Encode(a.body) //nolint
			}
		}
		enc.Encode(Request{ID: id, Command: CmdClose}) //nolint
		return buf.String()
	}()

	ctx := context.Background()
	b.SetBytes(totalBytes)
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()

		dir, err := os.MkdirTemp("", "gocacheprog-bench-*")
		if err != nil {
			b.Fatal(err)
		}
		storage := NewMetricsStorage(NewFileSystemStorage(dir), 0)

		// seed the "warm" portion of the cache
		for j := 0; j < numPreSeeded; j++ {
			a := actions[j]
			storage.Put(ctx, PutRequest{ //nolint
				Key:      hex.EncodeToString(a.actionID),
				OutputID: a.outputID,
				Body:     bytes.NewReader(a.body),
				BodySize: int64(len(a.body)),
			})
		}

		app := NewApp(strings.NewReader(stream), io.Discard, hex.EncodeToString, storage)
		b.StartTimer()

		app.Run(ctx)

		b.StopTimer()
		os.RemoveAll(dir)
	}
}
