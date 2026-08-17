.PHONY: build run seed reset bench test test-e2e clean

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

# End-to-end tests: build and run the real server binary, hit it over HTTP.
# Guarded by the `e2e` build tag so they stay out of the fast `test` target.
test-e2e:
	go test -tags e2e -count=1 ./e2e/...

clean:
	rm -rf bin data
