GO=go
GOFMT=gofmt
PROTOC=protoc
DELETE=rm
BINARY=coolbeans
# go source files, ignore vendor directory
SRC = $(shell find . -type f -name '*.go' -not -path "./vendor/*")
# current git version short-hash
VER = $(shell git rev-parse --short HEAD)
TAG = "$(BINARY):$(VER)"

info:
	@echo "---------------------------------------------------------------"
	@echo
	@echo " build          generate a build target to local setup         "
	@echo " clean          clean up bin/                                  "
	@echo " fmt            format using go fmt                            "
	@echo " genstr         generate enum string values for enums          "
	@echo " release/darwin generate a darwin target build                 "
	@echo " release/linux  generate a linux target build                  "
	@echo " run            build & run beanstalkd locally                 "
	@echo " test           run unit-tests                                 "
	@echo " testv          run unit-tests verbose                         "
	@echo 
	@echo "---------------------------------------------------------------" 

build: clean fmt protoc
	$(GO) build -o bin/$(BINARY) -v main.go

.PHONY: protoc
protoc:
	$(PROTOC) -I api/v1 api/v1/*.proto --go_out=plugins=grpc:api/v1

release/%: clean fmt
	@echo "build GOOS: $(subst release/,,$@) & GOARCH: amd64"
	GOOS=$(subst release/,,$@) GOARCH=amd64 $(GO) build -o bin/$(subst release/,,$@)/$(BINARY) -v main.go

run: build
	./bin/$(BINARY)

test: clean fmt
	$(GO) test ./...

testv: clean fmt
	$(GO) test -v ./...

testc: clean fmt
	# ToDo check out https://github.com/ory/go-acc
	./coverage_test.sh 
	$(GO) tool cover -html=coverage.txt

fmt:
	$(GOFMT) -l -w $(SRC)

.PHONY: clean
clean:
	$(DELETE) -rf bin/
	$(GO) clean -cache

