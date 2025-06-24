install:
	go install go.uber.org/mock/mockgen@v0.5.2

generate_mocks:
	rm mocks.go
	mockgen -package main -destination mocks.go gocacheprog Storage
