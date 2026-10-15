BINARY := proxy-relay
VERSION := 0.2.0
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
	@# NOTE: the android/arm64 binary below is built with CGO disabled and is
	@# NOT usable from Termux: Go's pure-Go resolver reads /etc/resolv.conf,
	@# which does not exist on Android, so every outbound dial fails with
	@#   lookup <host> on [::1]:53: read udp ...: connection refused
	@# Termux users must build natively on the device instead:
	@#   go build -o proxy-relay .      # uses Android's cgo resolver
	GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-android-arm64 .
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-windows-amd64.exe .
	@echo "Binaries in dist/"
	@ls -lh dist/

clean:
	rm -rf dist/ $(BINARY)
