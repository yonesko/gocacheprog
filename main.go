package main

import (
	"bufio"
	"encoding/base64"
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
	debug = flag.Bool("debug", false, "enable debug output")
	//dry = flag.Bool("debug", false, "enable debug output")
	tempDir = "/Users/gdanichev/GolandProjects/tsum/md/gocacheprog/temp"
	//tempDir = os.TempDir()
)

func main() {
	flag.Parse()
	//
	resp(Response{KnownCommands: []Cmd{CmdGet, CmdPut, CmdClose}}, nil)
	//
	reader := bufio.NewReader(os.Stdin)
	for {
		line := must(reader.ReadString('\n'))
		if *debug {
			fmt.Fprintln(os.Stderr, "> "+line)
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		var request Request
		must0(json.Unmarshal([]byte(line), &request))

		if request.Command == CmdGet {
			resp(get(request))
			continue
		}
		if request.Command == CmdPut {
			resp(put(request))
			continue
		}
		if request.Command == CmdClose {
			resp(Response{ID: request.ID}, nil)
			must0(os.Stdout.Close())
			os.Exit(0)
		}
	}
}

func toBase64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

func put(req Request) (Response, error) {
	if req.ActionID == nil || len(req.ActionID) == 0 {
		return Response{ID: req.ID}, errors.New("invalid action id")
	}
	diskPath := path.Join(tempDir, base64.StdEncoding.EncodeToString(req.ActionID))
	file, err := os.Create(diskPath)
	if err != nil {
		return Response{ID: req.ID}, err
	}
	defer file.Close()
	_, err = file.Write(req.OutputID)
	if err != nil {
		return Response{ID: req.ID}, err
	}
	//_, err = file.WriteString("\n")
	//if err != nil {
	//	return Response{ID: req.ID}, err
	//}
	written, err := io.CopyN(file, os.Stdin, req.BodySize+1)
	if err != nil {
		return Response{ID: req.ID}, err
	}
	if *debug {
		fmt.Fprintf(os.Stderr, "written %d to %s \n", written, diskPath)
	}
	return Response{ID: req.ID, DiskPath: diskPath}, nil
}

func get(req Request) (Response, error) {
	diskPath := path.Join(tempDir, base64.StdEncoding.EncodeToString(req.ActionID))
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
