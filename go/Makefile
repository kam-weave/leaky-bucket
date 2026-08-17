.PHONY: build run seed reset bench test test-e2e test-mutation test-all clean

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

# Mutation testing: mutate the limiter's source and confirm the tests catch it.
# gremlins is tracked as a go tool dependency (see `tool` in go.mod), so `go tool`
# builds it on demand — no separate install needed.
# GOFLAGS=-count=1 disables the go test result cache so gremlins measures a real
# baseline time — otherwise a cached (near-zero) baseline yields spurious timeouts.
test-mutation:
	GOFLAGS=-count=1 go tool gremlins unleash ./internal/ratelimit/

# Run every Go test level: unit + integration (with the race detector), e2e, and
# mutation. Slower than `make test`; use it before a release or a review.
test-all:
	go test -race ./...
	$(MAKE) test-e2e
	$(MAKE) test-mutation

clean:
	rm -rf bin data
