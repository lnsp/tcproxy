VERSION:=$(shell git describe --tags --always --dirty --match=v* 2> /dev/null || \
			cat $(CURDIR)/.version 2> /dev/null || echo v0)

all: build

.PHONY: bin-folder
bin-folder:
	mkdir -p ./bin

.PHONY: test
test:
	go test -v ./...

.PHONY: build
build: bin-folder
	go build -o bin/tcproxy -ldflags "-X ./Version=$(VERSION)"

.PHONY: install
install: build
	cp bin/tcproxy /usr/local/bin/tcproxy

.PHONY: clean
clean:
	rm -r bin/