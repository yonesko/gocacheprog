package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
)

type (
	index struct {
		OutputID []byte
		Size     int64
	}
	fileSystemStorage struct {
		dir string
	}
)

func NewFileSystemStorage(dir string) Storage {
	must0(os.MkdirAll(dir, 0755))
	return &fileSystemStorage{dir: dir}
}

func (f fileSystemStorage) Get(ctx context.Context, key string) (GetResponse, bool, error) {
	diskPathBody, diskPathIndex := f.fileNames(key)
	if isFileExists(diskPathBody) && isFileExists(diskPathIndex) {
		var ind index
		must0(json.Unmarshal(must(os.ReadFile(diskPathIndex)), &ind))
		diskPathBody = must(filepath.Abs(diskPathBody))
		return GetResponse{OutputID: ind.OutputID, DiskPath: diskPathBody, BodySize: ind.Size}, true, nil
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

func (f fileSystemStorage) fileNames(key string) (diskPathBody, diskPathIndex string) {
	diskPathBody = path.Join(f.dir, key+"-o")
	diskPathIndex = path.Join(f.dir, key+"-i")
	return diskPathBody, diskPathIndex
}

func (f fileSystemStorage) Put(ctx context.Context, request PutRequest) (string, error) {
	if len(request.Key) == 0 {
		return "", errors.New("empty key")
	}
	diskPathBody, diskPathIndex := f.fileNames(request.Key)
	bodyFile := must(os.Create(diskPathBody))
	indexFile := must(os.Create(diskPathIndex))
	defer bodyFile.Close()
	defer indexFile.Close()
	written := must(io.Copy(bodyFile, request.Body))
	must(indexFile.Write(must(json.Marshal(index{
		OutputID: request.OutputID,
		Size:     request.BodySize,
	}))))
	if written != request.BodySize {
		return "", fmt.Errorf("file %q size mismatch: %d != %d", diskPathBody, written, request.BodySize)
	}
	return must(filepath.Abs(diskPathBody)), nil
}

func (f fileSystemStorage) Close(ctx context.Context) error {
	return nil
}
