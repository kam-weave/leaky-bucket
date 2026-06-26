.PHONY: build-go run-go seed-go reset-go bench-go test-go clean-go
.PHONY: build-java run-java seed-java reset-java test-java clean-java
.PHONY: clean

# Go targets
build-go:
	$(MAKE) -C go build

run-go:
	$(MAKE) -C go run

seed-go:
	$(MAKE) -C go seed

reset-go:
	$(MAKE) -C go reset

bench-go:
	$(MAKE) -C go bench

test-go:
	$(MAKE) -C go test

clean-go:
	$(MAKE) -C go clean

# Java targets
build-java:
	$(MAKE) -C java build

run-java:
	$(MAKE) -C java run

seed-java:
	$(MAKE) -C java seed

reset-java:
	$(MAKE) -C java reset

test-java:
	$(MAKE) -C java test

clean-java:
	$(MAKE) -C java clean

# Combined targets
clean: clean-go clean-java
