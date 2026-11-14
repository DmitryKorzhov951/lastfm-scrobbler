BINARY  := lastfm-scrobbler
PKG     := ./cmd/lastfm-scrobbler
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -H=windowsgui -X github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/version.Version=$(VERSION)
WINDIR  := bin/windows

.PHONY: all windows dist-windows winres test race cover vet fmt lint clean

all: windows

windows:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' \
		-o $(WINDIR)/$(BINARY).exe $(PKG)
	cp third_party/webview2/WebView2Loader.dll $(WINDIR)/
	cp third_party/webview2/LICENSE.txt $(WINDIR)/WebView2Loader.LICENSE.txt
	@echo "Готово: $(WINDIR)/$(BINARY).exe и WebView2Loader.dll"

dist-windows: windows
	cp README.md $(WINDIR)/README.md
	cd $(WINDIR) && python3 -m zipfile -c ../$(BINARY)-$(VERSION)-windows-amd64.zip \
		$(BINARY).exe WebView2Loader.dll WebView2Loader.LICENSE.txt README.md
	@echo "Архив: bin/$(BINARY)-$(VERSION)-windows-amd64.zip"

winres:
	cd cmd/lastfm-scrobbler && go run github.com/tc-hib/go-winres@v0.3.3 make --arch amd64

test:
	go test ./...

race:
	go test -race ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

vet:
	go vet ./...
	GOOS=windows go vet ./...

fmt:
	gofmt -l -w .

lint: vet
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run || \
		echo "golangci-lint не установлен — пропущено"

clean:
	rm -rf bin coverage.out
