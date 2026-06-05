.PHONY: run test lint build clean coverage

run:
	go run main.go

test:
	go test -v -race -count=1 ./...

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

build:
	go build -o agentic-commerce .

clean: 
	rm -f agentic-commerce coverage.out coverage.html

lint:
	golangci-lint run ./...
