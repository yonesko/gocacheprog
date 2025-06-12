package main

type storage interface {
	Get(key string) ([]byte, error)
}
