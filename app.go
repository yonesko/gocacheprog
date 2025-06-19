package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
)

type (
	App struct {
		inputReader  io.Reader
		outputWriter io.Writer
		keyConverter func(src []byte) string
		storage      Storage
	}
)

func (a App) Run(ctx context.Context) error {
	reader := json.NewDecoder(bufio.NewReader(a.inputReader))
	//handshake
	a.resp(Response{KnownCommands: []Cmd{CmdGet, CmdPut, CmdClose}}, nil)
	//
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
				var body []byte
				err := reader.Decode(&body)
				if err != nil {
					a.resp(Response{ID: request.ID}, err)
					continue
				}
				request.Body = bytes.NewReader(body)
			} else {
				request.Body = bytes.NewBuffer(nil)
			}
			diskPath, err := a.storage.Put(ctx, PutRequest{
				Key:      a.keyConverter(request.ActionID),
				OutputID: request.OutputID,
				Body:     request.Body,
				BodySize: request.BodySize,
			})
			a.resp(Response{ID: request.ID, DiskPath: diskPath}, err)
			continue
		}

		if request.Command == CmdGet {
			entry, ok, err := a.storage.Get(ctx, a.keyConverter(request.ActionID))
			a.resp(Response{
				ID:       request.ID,
				Miss:     !ok,
				DiskPath: entry.DiskPath,
				OutputID: entry.OutputID,
				Size:     entry.BodySize,
			}, err)
			continue
		}
		if request.Command == CmdClose {
			err := a.storage.Close(ctx)
			a.resp(Response{ID: request.ID}, err)
			break
		}
	}
	return nil
}

func (a App) resp(response Response, err error) {
	if err != nil {
		response.Err = err.Error()
	}
	b := must(json.Marshal(response))
	must(a.outputWriter.Write(b))
	must(a.outputWriter.Write([]byte{'\n'}))
}

func NewApp(inputReader io.Reader, outputWriter io.Writer, keyConverter func(src []byte) string, storage Storage) App {
	return App{
		inputReader:  inputReader,
		outputWriter: outputWriter,
		keyConverter: keyConverter,
		storage:      storage,
	}
}
