# Makefile for the Docker image quay.io/glerchundi/renderizr
# MAINTAINER: Gorka Lerchundi Osa <glertxundi@gmail.com>
# If you update this image please bump the tag value before pushing.

.PHONY: all changelog build test static container push clean

VERSION = 0.1.0
PREFIX = quay.io/glerchundi

all: build

changelog:
	@FROM=$$FROM; \
	TO=$${TO:-HEAD}; \
	test -n "$$FROM" || { echo "missing FROM environment variable" 1>&2 ; exit 1; }; \
	git --no-pager log --merges --format="%h %b" $$FROM..$$TO

build:
	@echo "Building renderizr..."
	gb build all

test:
	@echo "Running tests..."
	gb test

static:
	@echo "Building renderizr (static)..."
	@ROOTPATH=$(shell pwd -P); \
	mkdir -p $$ROOTPATH/bin; \
	cd $$ROOTPATH/src/github.com/glerchundi/renderizr; \
	GOPATH=$$ROOTPATH/vendor:$$ROOTPATH \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
	  go build \
	    -a -tags netgo -installsuffix cgo -ldflags '-extld ld -extldflags -static' -a -x \
	    -o $$ROOTPATH/bin/renderizr-linux-amd64 \
	    . \
	; \
	GOPATH=$$ROOTPATH/vendor:$$ROOTPATH \
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 \
	  go build \
	    -a -tags netgo -installsuffix cgo -ldflags '-extld ld -extldflags -static' -a -x \
	    -o $$ROOTPATH/bin/renderizr-darwin-amd64 \
	    .

container: static
	docker build -t $(PREFIX)/renderizr:$(VERSION) .

push: container
	docker push $(PREFIX)/renderizr:$(VERSION)

clean:
	rm -f bin/renderizr*
