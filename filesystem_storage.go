package main

import (
	"bufio"
	"context"
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

func (f fileSystemStorage) Get(ctx context.Context, key string) (Entry, bool, error) {
	diskPath := path.Join(f.dir, key)
	if _, err := os.Stat(diskPath); err == nil {
		file, err := os.Open(diskPath)
		if err != nil {
			return Entry{}, false, err
		}
		defer file.Close()
		outputID, err := bufio.NewReader(file).ReadBytes('\n')
		if err != nil {
			return Entry{}, false, err
		}
		outputID = outputID[:len(outputID)-1] //remove \n

		return Entry{OutputID: outputID, DiskPath: diskPath}, true, nil
	} else if os.IsNotExist(err) {
		return Entry{}, false, nil
	} else {
		return Entry{}, false, err
	}
}

func (f fileSystemStorage) Put(ctx context.Context, key string, outputID []byte, body io.Reader) (string, error) {
	if len(key) == 0 {
		return "", errors.New("empty key")
	}
	diskPath := path.Join(f.dir, key)
	file, err := os.Create(diskPath)
	if err != nil {
		return "", fmt.Errorf("creating file: %w", err)
	}
	defer file.Close()
	_, err = file.Write(outputID)
	if err != nil {
		return "", fmt.Errorf("writing to file: %w", err)
	}
	_, err = file.WriteString("\n")
	if err != nil {
		return "", fmt.Errorf("writing to file: %w", err)
	}
	_, err = io.Copy(file, body)
	if err != nil {
		return "", fmt.Errorf("writing to file: %w", err)
	}
	return diskPath, nil

}

func (f fileSystemStorage) Close(ctx context.Context) error {
	return nil
}
