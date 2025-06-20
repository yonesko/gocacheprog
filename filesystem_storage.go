package main

import (
	"bytes"
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
		defer fileIndex.Close()
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
	err := writeFileAtomically(diskPathBody, request.Body)
	if err != nil {
		return "", fmt.Errorf("error creating body file %s: %w", request.Key, err)
	}
	indexBytes, err := json.Marshal(index{
		OutputID: request.OutputID,
		Size:     request.BodySize,
	})
	if err != nil {
		return "", fmt.Errorf("error marshalling index: %w %s", err, request.Key)
	}
	err = writeFileAtomically(diskPathIndex, bytes.NewReader(indexBytes))
	if err != nil {
		return "", fmt.Errorf("error creating index file %s: %w", request.Key, err)
	}
	return filepath.Abs(diskPathBody)
}

func writeFileAtomically(path string, body io.Reader) error {
	file, err := os.CreateTemp("", "*")
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, body)
	if err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func (f fileSystemStorage) Close(ctx context.Context) error {
	return nil
}
