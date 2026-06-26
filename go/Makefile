.PHONY: build run seed reset bench test clean

build:
	go build -o bin/server ./cmd/server

run:
	go run ./cmd/server

seed:
	go run ./cmd/server --seed

reset:
	rm -rf data
	go run ./cmd/server --seed

bench:
	@mkdir -p data
	go test -bench=. -run=^$$ 2>data/bench.log

test:
	go test ./...

clean:
	rm -rf bin data
