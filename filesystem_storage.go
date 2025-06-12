package main

import (
	"context"
	"encoding/hex"
	"errors"
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

func (f fileSystemStorage) Get(ctx context.Context, key string) (Entry, bool, error) {
	diskPathBody := path.Join(f.dir, key+"-o")
	diskPathIndex := path.Join(f.dir, key+"-i")
	if isFileExists(diskPathBody) && isFileExists(diskPathIndex) {
		outputID := must(hex.DecodeString(string(must(os.ReadFile(diskPathIndex)))))
		return Entry{OutputID: outputID, DiskPath: diskPathBody}, true, nil
	}
	return Entry{}, false, nil
}

func isFileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}

func (f fileSystemStorage) Put(ctx context.Context, key string, outputID []byte, body io.Reader) (string, error) {
	if len(key) == 0 {
		return "", errors.New("empty key")
	}
	diskPathBody := path.Join(f.dir, key+"-o")
	diskPathIndex := path.Join(f.dir, key+"-i")
	bodyFile := must(os.Create(diskPathBody))
	indexFile := must(os.Create(diskPathIndex))
	defer bodyFile.Close()
	defer indexFile.Close()
	must(io.Copy(bodyFile, body))
	must(indexFile.WriteString(hex.EncodeToString(outputID)))
	//TODO compare sizes
	return diskPathBody, nil
}

func (f fileSystemStorage) Close(ctx context.Context) error {
	return nil
}
