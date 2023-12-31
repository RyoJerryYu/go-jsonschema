PKG := .
CMD := $(PKG)/cmd/jsonschemagen
PACKAGE := github.com/RyoJerryYu/go-jsonschema
BIN := jsonschemagen

CODECOVFLAGS=-coverprofile=coverage.txt -covermode=atomic -coverpkg=${PACKAGE}

# Build

.PHONY: all clean

all: clean $(BIN)

$(BIN):
	@echo "+ Building $@"
	CGO_ENABLED="0" go build -v -o $@ $(CMD)

clean:
	@echo "+ Cleaning $(PKG)"
	go clean -i $(PKG)/...
	rm -f $(BIN)
	rm -rf test/*_gen

# Test

# generate sources
JSON := $(wildcard test/*.json)
GENERATED_SOURCE := $(patsubst %.json,%_gen/generated.go,$(JSON))
test/%_gen/generated.go: test/%.json 
	@echo "\n+ Generating code for $@, from $^"
	@D=$(shell echo $^ | sed 's/.json/_gen/'); \
	[ ! -d $$D ] && mkdir -p $$D || true
	./jsonschemagen -o $@ -n $(shell echo $^ | sed 's/test\///; s/.json//')  -s $^

.PHONY: test codecheck fmt lint vet

test: $(BIN) $(GENERATED_SOURCE)
	@echo "\n+ Executing tests for $(PKG)"
	go test -v -race ${CODECOVFLAGS} $(PKG)/...
    

codecheck: fmt lint vet

fmt:
	@echo "+ go fmt"
	go fmt $(PKG)/...

lint: $(GOPATH)/bin/golint
	@echo "+ go lint"
	golint -min_confidence=0.1 $(PKG)/...

$(GOPATH)/bin/golint:
	go install golang.org/x/lint/golint@latest

vet:
	@echo "+ go vet"
	go vet $(PKG)/...
