# Pomen - plugin for Intermasq
# Copyright (C) 2026 AlexRus1234
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU Affero General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
# GNU Affero General Public License for more details.
#
# You should have received a copy of the GNU Affero General Public License
# along with this program. If not, see <https://www.gnu.org/licenses/>.

# Makefile for Pomen plugin. Mirrors the CI steps locally.
# Usage:
#   make            # = make build
#   make build      # cross-compile pomen-linux (static, stripped)
#   make run        # build + run with PLUGIN_SOCKET=/tmp/pomen.sock
#   make test       # go test with -race
#   make cover      # coverage report -> coverage.out
#   make lint       # go vet + gofmt check
#   make clean      # remove built artifacts

GO          ?= go
BINARY      := pomen
BINARY_LINUX := pomen-linux
VERSION     := $(shell $(GO) run -ldflags '-X pomen/internal/version.Version=dev' . --version 2>/dev/null || echo dev)
LDFLAGS     := -s -w -X pomen/internal/version.Version=$(VERSION)

.PHONY: all build build-linux run test test-race cover vet fmt lint clean

all: build

build:
	$(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY) .

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
	$(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY_LINUX) .

run: build
	PLUGIN_SOCKET=/tmp/pomen.sock STATE_FILE=/tmp/pomen-routes.json VMS_FILE=/tmp/pomen-vms.json \
	./$(BINARY)

test:
	$(GO) test ./... -count=1

test-race:
	CGO_ENABLED=1 $(GO) test ./... -race -count=1

cover:
	CGO_ENABLED=1 $(GO) test ./... -count=1 -coverprofile coverage.out
	$(GO) tool cover -func coverage.out | tail -n 1

vet:
	$(GO) vet ./...

fmt:
	@gofmt -l .
	@echo "gofmt check done (empty = OK)"

lint: vet
	@FMT_OUT="$$(gofmt -l .)"; \
	if [ -n "$$FMT_OUT" ]; then \
		echo "Files need gofmt:"; echo "$$FMT_OUT"; exit 1; \
	fi

clean:
	rm -f $(BINARY) $(BINARY_LINUX) coverage.out
