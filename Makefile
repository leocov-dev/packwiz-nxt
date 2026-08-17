GOFMT_FILES?=$$(find . -type f -name '*.go' -not -path './vendor/*' -not -path './.git/*')

default: dev

test:
	@rm -f .coverage/coverage.out .coverage/coverage.html
	@go test -v -coverpkg=./... -coverprofile=.coverage/coverage.out ./...
	@go tool cover -html=.coverage/coverage.out -o .coverage/coverage.html

dev: tidy fmt
	@go build -race -o "bin/packwiz" -tags=netgo -ldflags="-extldflags=-static -X 'main.CfApiKey=$(CF_API_KEY)' -X 'main.GhApiKey=$(GH_API_KEY)'" .

fmt:
	@gofmt -w $(GOFMT_FILES)

lint:
	@gofmt -e -l $(GOFMT_FILES)

tidy:
	@go mod tidy

generate:
	@go run github.com/vektra/mockery/v2@v2.53.6

.NOTPARALLEL:

.PHONY: fmtcheck fmt tidy test generate