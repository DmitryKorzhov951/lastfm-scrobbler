BINARY  := lastfm-scrobbler
PKG     := ./cmd/lastfm-scrobbler
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/version.Version=$(VERSION)

PREFIX   ?= $(HOME)/.local
BINDIR   ?= $(PREFIX)/bin
UNITDIR  ?= $(HOME)/.config/systemd/user
UNIT     := packaging/systemd/$(BINARY).service

.PHONY: all build windows dist-windows winres test vet fmt lint race cover clean install uninstall service enable disable logs

all: build

build:
	go build -trimpath -ldflags '$(LDFLAGS)' -o bin/$(BINARY) $(PKG)

WINDIR := bin/windows
windows:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' \
		-o $(WINDIR)/$(BINARY).exe $(PKG)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath \
		-ldflags '$(LDFLAGS) -H=windowsgui -X main.backgroundBuild=1' \
		-o $(WINDIR)/$(BINARY)w.exe $(PKG)
	cp third_party/webview2/WebView2Loader.dll $(WINDIR)/
	cp third_party/webview2/LICENSE.txt $(WINDIR)/WebView2Loader.LICENSE.txt
	@echo "Готово: $(WINDIR)/$(BINARY).exe, $(WINDIR)/$(BINARY)w.exe и WebView2Loader.dll"

dist-windows: windows
	cp README.md $(WINDIR)/README.md
	cd $(WINDIR) && python3 -m zipfile -c ../$(BINARY)-$(VERSION)-windows-amd64.zip \
		$(BINARY).exe $(BINARY)w.exe WebView2Loader.dll WebView2Loader.LICENSE.txt README.md
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
		echo "golangci-lint не установлен — пропущено (pacman -S golangci-lint)"

clean:
	rm -rf bin coverage.out

install: build
	install -Dm755 bin/$(BINARY) $(DESTDIR)$(BINDIR)/$(BINARY)
	@echo "Установлено: $(DESTDIR)$(BINDIR)/$(BINARY)"
	@case ":$$PATH:" in *":$(BINDIR):"*) ;; \
		*) echo "ВНИМАНИЕ: $(BINDIR) отсутствует в PATH";; esac

uninstall: disable
	rm -f $(DESTDIR)$(BINDIR)/$(BINARY)
	rm -f $(UNITDIR)/$(BINARY).service
	systemctl --user daemon-reload 2>/dev/null || true
	@echo "Удалено. Настройки и очередь остались в ~/.config/$(BINARY) и ~/.local/state/$(BINARY)"

service: install
	install -d $(UNITDIR)
	sed 's|^ExecStart=.*|ExecStart=$(BINDIR)/$(BINARY) run|' $(UNIT) > $(UNITDIR)/$(BINARY).service
	systemctl --user daemon-reload
	@echo "Юнит установлен: $(UNITDIR)/$(BINARY).service"
	@echo "Включить: make enable"

enable:
	systemctl --user enable --now $(BINARY).service
	systemctl --user status --no-pager $(BINARY).service || true

disable:
	-systemctl --user disable --now $(BINARY).service 2>/dev/null

logs:
	journalctl --user -u $(BINARY).service -f
