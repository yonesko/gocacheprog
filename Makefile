install:
	go install go.uber.org/mock/mockgen@v0.5.2

generate_mocks:
	rm mocks.go
	mockgen -package main -destination mocks.go gocacheprog Storage

benchmark:
	go test -v -run=^$$  -count 15 -bench=. gocacheprog > bench.txt

benchmark_compare:
 	git show HEAD^:bench.txt > old_bench.txt
	benchmark
	benchstat old_bench.txt bench.txt
