package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

var (
	debug = flag.Bool("debug", false, "enable debug output")
	//dry = flag.Bool("debug", false, "enable debug output")
)

func main() {
	flag.Parse()
	//
	resp(Response{KnownCommands: []Cmd{CmdGet, CmdPut, CmdClose}}, nil)
	//
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		if scanner.Err() != nil {
			panic(scanner.Err())
		}
		if *debug {
			fmt.Fprintln(os.Stderr, "> "+scanner.Text())
		}
		if strings.TrimSpace(string(scanner.Bytes())) == "" {
			continue
		}
		var request Request
		must0(json.Unmarshal(scanner.Bytes(), &request))

		if request.Command == CmdGet {
			resp(get(request))
			continue
		}
		if request.Command == CmdPut {
			for scanner.Scan() {
				if scanner.Err() != nil {
					panic(scanner.Err())
				}
				if strings.TrimSpace(string(scanner.Bytes())) == "" {
					continue
				}

				bodyBase64 := scanner.Bytes()
				if *debug {
					fmt.Fprintf(os.Stderr, "recieved %d bytes of body: %s...\n", len(bodyBase64), string(bodyBase64[:min(10, len(bodyBase64))]))
				}
				break
			}
			resp(Response{ID: request.ID}, nil)
		}
		if request.Command == CmdClose {
			resp(Response{ID: request.ID}, nil)
			must0(os.Stdout.Close())
			os.Exit(0)
		}
	}
}

type handlerFunc func(req Request) (Response, error)

//func router(req Request) (Response, error) {
//	handlers := map[Cmd]handlerFunc{
//		CmdPut: put,
//		CmdGet: get,
//	}
//	h, ok := handlers[req.Command]
//	if !ok {
//		return Response{}, fmt.Errorf("unknown command: %s", req.Command)
//	}
//	return h(req)
//}

func put(req Request) (Response, error) {
	return Response{}, nil
}

func get(req Request) (Response, error) {
	return Response{ID: req.ID, Miss: true}, nil
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
	bytes := must(json.Marshal(response))
	_, _ = os.Stdout.Write(bytes)
	if *debug {
		os.Stderr.WriteString(fmt.Sprintf("< %s\n", string(bytes)))
	}

}
