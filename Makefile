
GO_SRC := $(shell find -type f -name '*.go' ! -path '*/vendor/*')
GO_PKG := $(shell find -type d ! -path '*/vendor/*' ! -path '.*')
VERSION ?= $(shell git describe --long --dirty)

all: style vet test tail_exporter.x86_64

tail_exporter.x86_64: $(GO_SRC)
	GOOS=linux GOARCH=amd64 go build -a \
	-ldflags "-extldflags '-static' -X main.Version=$(shell git describe --long --dirty)" \
	-o tail_exporter.x86_64 .

vet:
	go vet

# Check code conforms to go fmt
style:
	! gofmt -s -l $(GO_SRC) 2>&1 | read 2>/dev/null

test:
	go test -v -covermode=count -coverprofile=cover.out

lint:
	golint $(GO_PKG)

# Format the code
fmt:
	gofmt -s -w $(GO_SRC)

.PHONY: test vet
