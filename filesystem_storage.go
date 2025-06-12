package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
)

type fileSystemStorage struct {
	dir string
}

func NewFileSystemStorage(dir string) Storage {
	must0(os.MkdirAll(dir, 0755))
	return &fileSystemStorage{dir: dir}
}

func (f fileSystemStorage) Get(ctx context.Context, key string) (GetResponse, bool, error) {
	diskPathBody := path.Join(f.dir, key+"-o")
	diskPathIndex := path.Join(f.dir, key+"-i")
	if isFileExists(diskPathBody) && isFileExists(diskPathIndex) {
		outputID := must(hex.DecodeString(string(must(os.ReadFile(diskPathIndex)))))
		return GetResponse{OutputID: outputID, DiskPath: diskPathBody}, true, nil
	}
	return GetResponse{}, false, nil
}

func isFileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}

func (f fileSystemStorage) Put(ctx context.Context, request PutRequest) (string, error) {
	if len(request.Key) == 0 {
		return "", errors.New("empty key")
	}
	diskPathBody := path.Join(f.dir, request.Key+"-o")
	diskPathIndex := path.Join(f.dir, request.Key+"-i")
	bodyFile := must(os.Create(diskPathBody))
	indexFile := must(os.Create(diskPathIndex))
	defer bodyFile.Close()
	defer indexFile.Close()
	written := must(io.Copy(bodyFile, request.Body))
	must(indexFile.WriteString(hex.EncodeToString(request.OutputID)))
	if written != request.BodySize {
		return "", fmt.Errorf("file %q size mismatch: %d != %d", diskPathBody, written, request.BodySize)
	}
	return diskPathBody, nil
}

func (f fileSystemStorage) Close(ctx context.Context) error {
	return nil
}
