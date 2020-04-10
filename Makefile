GO=go
GOFMT=gofmt
GOREMAN=goreman
PROTOC=protoc
DELETE=rm
BINARY=coolbeans
# go source files, ignore vendor directory
SRC = $(shell find . -type f -name '*.go' -not -path "./vendor/*")
# current git version short-hash
VER = $(shell git rev-parse --short HEAD)
TAG = "$(BINARY):$(VER)"

info:
	@echo "_______________________________________________________________"
	@echo
	@echo " target         Description.                                   "
	@echo "_______________________________________________________________"
	@echo
	@echo " build          generate a build target to local setup         "
	@echo " clean          clean up bin/ & go test cache                  "
	@echo " fmt            format go code files using go fmt              "
	@echo " generate       generate enum-strings & test-fakes go files    "
	@echo " protoc         compile proto files to generate go files       "
	@echo " release/darwin generate a darwin target build                 "
	@echo " release/linux  generate a linux target build                  "
	@echo " run-single     run a single node cluster w/ beanstalkd proxy  "
	@echo " run-cluster    run a three node cluster w/ beanstalkd proxy   "
	@echo " test           run unit-tests                                 "
	@echo " testc          run unit-tests w/ coverage                     "
	@echo " testv          run unit-tests verbose                         "
	@echo " tidy           clean up go module file                        "
	@echo "_______________________________________________________________"

build: clean fmt protoc test
	$(GO) build -o bin/$(BINARY) -v main.go


.PHONY: clean
clean:
	$(DELETE) -rf bin/
	$(GO) clean -cache


.PHONY: fmt
fmt:
	$(GOFMT) -l -w $(SRC)


# tools deps to generate code (stringer...)
# https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
.PHONY: generate
generate:
	$(GO) generate ./...


.PHONY: protoc
protoc:
	$(PROTOC) -I api/v1 api/v1/*.proto --go_out=plugins=grpc:api/v1


release/%: clean fmt generate protoc tidy test
	@echo "build GOOS: $(subst release/,,$@) & GOARCH: amd64"
	GOOS=$(subst release/,,$@) GOARCH=amd64 $(GO) build -o bin/$(subst release/,,$@)/$(BINARY) -v main.go


.PHONY: run-single
run-single:
	$(GO) get github.com/mattn/goreman
	$(GOREMAN) -f Procfiles/dev.procfile start

.PHONY: run-cluster
run-cluster:
	$(GO) get github.com/mattn/goreman
	$(GOREMAN) -f Procfiles/dev-cluster.procfile start


# test w/ race detector on always
# https://golang.org/doc/articles/race_detector.html#Typical_Data_Races
.PHONY: test
test: clean fmt
	$(GO) test -race ./...


.PHONY: testv
testv: clean fmt
	$(GO) test -v -race ./...


.PHONY: testc
testc: clean fmt
	$(GO) get github.com/ory/go-acc
	./coverage_test.sh 
	$(GO) tool cover -html=coverage.txt

.PHONY: tidy
tidy:
	$(GO) mod tidy
