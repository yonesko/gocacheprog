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
		fileIndex, err := os.Open(diskPathIndex)
		if err != nil {
			return GetResponse{}, false, fmt.Errorf("error opening index file %s: %w", key, err)
		}
		err = json.NewDecoder(fileIndex).Decode(&ind)
		if err != nil {
			return GetResponse{}, false, fmt.Errorf("failed to unmarshal file %s: %w", key, err)
		}

		absDiskPathBody, err := filepath.Abs(diskPathBody)
		if err != nil {
			return GetResponse{}, false, fmt.Errorf("failed to determine absolute path for %s: %w", key, err)
		}
		return GetResponse{OutputID: ind.OutputID, DiskPath: absDiskPathBody, BodySize: ind.Size}, true, nil
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
	fileBody, err := os.Create(diskPathBody)
	if err != nil {
		return "", fmt.Errorf("error creating file %s: %w", request.Key, err)
	}
	defer fileBody.Close()
	fileIndex, err := os.Create(diskPathIndex)
	if err != nil {
		return "", fmt.Errorf("error creating file %s: %w", request.Key, err)
	}
	defer fileIndex.Close()
	written, err := io.Copy(fileBody, request.Body)
	if err != nil {
		return "", fmt.Errorf("error writing file %s: %w", request.Key, err)
	}
	indexBytes, err := json.Marshal(index{
		OutputID: request.OutputID,
		Size:     request.BodySize,
	})
	if err != nil {
		return "", fmt.Errorf("error marshalling index: %w %s", err, request.Key)
	}
	_, err = fileIndex.Write(indexBytes)
	if err != nil {
		return "", fmt.Errorf("error writing index: %w %s", err, request.Key)
	}
	if written != request.BodySize {
		return "", fmt.Errorf("file %s size mismatch: %d != %d", request.Key, written, request.BodySize)
	}
	return filepath.Abs(diskPathBody)
}

func (f fileSystemStorage) Close(ctx context.Context) error {
	return nil
}
