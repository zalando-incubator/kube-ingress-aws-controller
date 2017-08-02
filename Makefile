.PHONY: clean check build.local build.linux build.osx build.docker build.push

BINARY        ?= kube-ingress-aws-controller
VERSION       ?= $(shell git describe --tags --always --dirty)
IMAGE         ?= registry-write.opensource.zalan.do/teapot/$(BINARY)
TAG           ?= $(VERSION)
GITHEAD       = $(shell git rev-parse --short HEAD)
GITURL        = $(shell git config --get remote.origin.url)
GITSTATUS     = $(shell git status --porcelain || echo "no changes")
SOURCES       = $(shell find . -name '*.go') aws/cftemplate.go
DOCKERFILE    ?= Dockerfile
GOPKGS        = $(shell go list ./... | grep -v /vendor/)
BUILD_FLAGS   ?= -v
LDFLAGS       ?= -X controller.version=$(VERSION) -w -s


default: build.local

clean:
	rm -rf build
	rm -f aws/cftemplate.go

test: aws/cftemplate.go
	go test -v -race -cover $(GOPKGS)

fmt:
	go fmt $(GOPKGS)

check:
	golint $(GOPKGS)
	go vet -v $(GOPKGS)

build.local: build/$(BINARY)
build.linux: build/linux/$(BINARY)
build.osx: build/osx/$(BINARY)

aws/cftemplate.go: aws/ingress-cf-template.yaml aws/cf.go
	go generate aws/cf.go

build/$(BINARY): $(SOURCES)
	go build -o build/$(BINARY) $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" .

build/linux/$(BINARY): $(SOURCES)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(BUILD_FLAGS) -o build/linux/$(BINARY) -ldflags "$(LDFLAGS)" .

build/osx/$(BINARY): $(SOURCES)
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(BUILD_FLAGS) -o build/osx/$(BINARY) -ldflags "$(LDFLAGS)" .

build.docker:
	docker build -t "$(IMAGE):$(TAG)" -f $(DOCKERFILE) .

build.push: build.docker
	docker push "$(IMAGE):$(TAG)"
