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
)

func main() {
	flag.Parse()
	//
	resp(Response{KnownCommands: []Cmd{CmdGet, CmdPut, CmdClose}})
	//
	handler := router
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		if *debug {
			os.Stderr.WriteString("> " + scanner.Text() + "\n")
		}
		if strings.TrimSpace(string(scanner.Bytes())) == "" {
			continue
		}
		var request Request
		err := json.Unmarshal(scanner.Bytes(), &request)
		if err != nil {
			resp(Response{ID: request.ID, Err: err.Error()})
			continue
		}

		response, ok, err := handler(request)
		if err != nil {
			resp(Response{ID: request.ID, Err: err.Error()})
			continue
		}
		if !ok {
			resp(Response{ID: request.ID, Err: "handler is not found"})
			continue
		}
		response.ID = request.ID
		resp(response)
	}
}

type handlerFunc func(req Request) (Response, bool, error)

func router(req Request) (Response, bool, error) {
	handlers := map[Cmd]handlerFunc{
		CmdPut: put,
		CmdGet: get,
	}
	h, ok := handlers[req.Command]
	if !ok {
		return Response{}, false, fmt.Errorf("unknown command: %s", req.Command)
	}
	return h(req)
}

func put(req Request) (Response, bool, error) {
	if req.Command == CmdPut {
		bodyBase64, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			return Response{}, false, err
		}
		if *debug {
			fmt.Fprintf(os.Stderr, "received %d bytes\n", len(bodyBase64))
		}
	}
	return Response{}, false, nil
}

func get(req Request) (Response, bool, error) {
	return Response{Miss: true}, true, nil
}

func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}
	return t
}

func resp(r Response) {
	bytes := must(json.Marshal(r))
	_, _ = os.Stdout.Write(bytes)
	if *debug {
		os.Stderr.WriteString(fmt.Sprintf("< %s\n", string(bytes)))
	}

}
