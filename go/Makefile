.PHONY: build run seed reset bench test test-e2e test-mutation clean

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
# Needs gremlins: go install github.com/go-gremlins/gremlins/cmd/gremlins@latest
# GOFLAGS=-count=1 disables the go test result cache so gremlins measures a real
# baseline time — otherwise a cached (near-zero) baseline yields spurious timeouts.
test-mutation:
	@GREMLINS=$$(command -v gremlins || echo "$$(go env GOPATH)/bin/gremlins"); \
	 if [ ! -x "$$GREMLINS" ]; then \
	   echo "gremlins not found. Install: go install github.com/go-gremlins/gremlins/cmd/gremlins@latest"; \
	   exit 1; \
	 fi; \
	 GOFLAGS=-count=1 "$$GREMLINS" unleash ./internal/ratelimit/

clean:
	rm -rf bin data
