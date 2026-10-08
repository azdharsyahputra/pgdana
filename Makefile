.PHONY: build run test clean

build:
	@echo "Building binary..."
	go build -o bin/server ./cmd/server

run: build
	@echo "Starting server..."
	./bin/server

test:
	@echo "Running tests..."
	go test -v ./...

clean:
	@rm -rf bin data
