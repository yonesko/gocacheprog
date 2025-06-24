install:
	go install go.uber.org/mock/mockgen@v0.5.2

generate_mocks:
	rm mocks.go
	mockgen -package main -destination mocks.go gocacheprog Storage

benchmark:
	git show HEAD:bench.txt > old_bench.txt
	go test -v -run=^$$  -count 15 -bench=. gocacheprog > bench.txt
	benchstat old_bench.txt bench.txt

