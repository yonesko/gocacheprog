package main

import (
	"bufio"
	"bytes"
	"container/list"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

var (
	debug   = flag.Bool("v", false, "enable verbose output")
	tempDir = "/Users/gdanichev/GolandProjects/tsum/md/gocacheprog/temp"
	//tempDir = os.TempDir()
)

type (
	handlerFunc func(req Request) (Response, error)
)

func main() {
	flag.Parse()
	//
	resp(Response{KnownCommands: []Cmd{CmdGet, CmdPut, CmdClose}}, nil)
	//
	statistics := &stat{}
	getFunc := metric(statistics, get)
	putFunc := metric(statistics, put)
	reader := bufio.NewReader(os.Stdin)
	var pendingPutRequests list.List
	pendingPutRequests.Init()
	for {
		line, err := reader.ReadBytes('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}
		if *debug {
			fmt.Fprint(os.Stderr, "> "+string(line))
		}
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var request Request
		if strings.HasPrefix(string(line), "{") {
			must0(json.Unmarshal(line, &request))
		} else {
			element := pendingPutRequests.Front()
			pendingPutRequests.Remove(element)
			request = element.Value.(Request)
			request.Body = bytes.NewReader(line[1 : len(line)-1]) //remove quotes.
			resp(putFunc(request))
			continue
		}

		if request.Command == CmdGet {
			resp(getFunc(request))
			continue
		}
		if request.Command == CmdPut {
			if request.BodySize == 0 {
				request.Body = strings.NewReader("")
				resp(putFunc(request))
				continue
			}
			pendingPutRequests.PushBack(request)
			continue
		}
		if request.Command == CmdClose {
			resp(Response{ID: request.ID}, nil)
			must0(os.Stdout.Close())
			fmt.Fprintf(os.Stderr, "\nstatistics: %+v\n", statistics)
			os.Exit(0)
		}
	}
}

func put(req Request) (Response, error) {
	if req.ActionID == nil || len(req.ActionID) == 0 {
		return Response{ID: req.ID}, errors.New("invalid action id")
	}
	diskPath := path.Join(tempDir, calcFileName(req.ActionID))
	file, err := os.Create(diskPath)
	if err != nil {
		return Response{ID: req.ID}, fmt.Errorf("creating file: %w", err)
	}
	defer file.Close()
	_, err = file.Write(req.OutputID)
	if err != nil {
		return Response{ID: req.ID}, fmt.Errorf("writing to file: %w", err)
	}
	_, err = file.WriteString("\n")
	if err != nil {
		return Response{ID: req.ID}, fmt.Errorf("writing to file: %w", err)
	}
	//written, err := io.CopyN(file, os.Stdin, req.BodySize+1)
	//TODO make buff less copy
	_, err = io.Copy(file, req.Body)
	if err != nil {
		return Response{ID: req.ID}, fmt.Errorf("writing to file: %w", err)
	}
	return Response{ID: req.ID, DiskPath: diskPath}, nil
}

func get(req Request) (Response, error) {
	diskPath := path.Join(tempDir, calcFileName(req.ActionID))
	if _, err := os.Stat(diskPath); err == nil {
		file, err := os.Open(diskPath)
		if err != nil {
			return Response{ID: req.ID}, err
		}
		defer file.Close()
		outputID, err := bufio.NewReader(file).ReadBytes('\n')
		if err != nil {
			return Response{ID: req.ID}, err
		}

		return Response{ID: req.ID, OutputID: outputID, DiskPath: diskPath}, nil
	} else if os.IsNotExist(err) {
		return Response{ID: req.ID, Miss: true}, nil
	} else {
		return Response{ID: req.ID}, err
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
	bytes := must(json.Marshal(response))
	_, _ = os.Stdout.Write(bytes)
	if *debug {
		os.Stderr.WriteString(fmt.Sprintf("< %s\n", string(bytes)))
	}

}

func calcFileName(data []byte) string {
	if len(data) == 0 {
		panic("calcFileName called with empty data")
	}
	return hex.EncodeToString(data)
}
