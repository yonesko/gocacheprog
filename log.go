package main

import (
	"io"
	"os"
)

type loggingReader struct {
	io.Reader
}

func newLoggingReader(reader io.Reader) io.Reader {
	return &loggingReader{Reader: reader}
}

func (l loggingReader) Read(p []byte) (n int, err error) {
	read, err := l.Reader.Read(p)
	if err != nil {
		return read, err
	}
	os.Stderr.Write(p[:read])
	return read, err
}

type loggingWriter struct {
	io.Writer
}

func newLoggingWriter(writer io.Writer) io.Writer {
	return &loggingWriter{Writer: writer}
}

func (l loggingWriter) Write(p []byte) (n int, err error) {
	write, err := l.Writer.Write(p)
	if err != nil {
		return write, err
	}
	os.Stderr.Write(p[:write])
	return write, err
}
