BINARY := proxy-relay
VERSION := 0.1.0
LDFLAGS := -s -w

.PHONY: all build clean cross

all: build

build:
	go build -ldflags="$(LDFLAGS)" -o $(BINARY) .

cross: clean
	GOOS=linux   GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64   .
	GOOS=linux   GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-linux-arm64   .
	GOOS=darwin  GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-darwin-amd64  .
	GOOS=darwin  GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64  .
	GOOS=android GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-android-arm64 .
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-windows-amd64.exe .
	@echo "Binaries in dist/"
	@ls -lh dist/

clean:
	rm -rf dist/ $(BINARY)
