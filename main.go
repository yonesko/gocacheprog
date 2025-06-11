package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

var (
	debug = true
)

func main() {
	//
	resp(Response{KnownCommands: []Cmd{CmdGet, CmdPut, CmdClose}})
	//
	handler := chain
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		if debug {
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

func chain(req Request) (Response, bool, error) {
	handlers := []handlerFunc{
		put,
		alwaysMiss,
	}
	for _, h := range handlers {
		response, ok, err := h(req)
		if err != nil {
			return Response{}, false, err
		}
		if !ok {
			continue
		}
		return response, true, nil
	}
	return Response{}, false, nil
}

func put(req Request) (Response, bool, error) {
	if req.ID == 0 {
		return Response{KnownCommands: []Cmd{CmdGet, CmdPut, CmdClose}}, true, nil
	}
	return Response{}, false, nil
}

func alwaysMiss(req Request) (Response, bool, error) {
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
	if debug {
		os.Stderr.WriteString(fmt.Sprintf("< %s\n", string(bytes)))
	}

}
