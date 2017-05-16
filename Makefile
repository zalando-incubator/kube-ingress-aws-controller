.PHONY: clean check build.local build.linux build.osx build.docker build.push

BINARY        ?= kube-aws-ingress-controller
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

test:
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

$(DOCKERFILE).upstream: $(DOCKERFILE)
	sed "s@UPSTREAM@$(shell $(shell head -1 $(DOCKERFILE) | sed -E 's@FROM (.*)/(.*)/(.*):.*@pierone latest \2 \3 --url \1@'))@" $(DOCKERFILE) > $(DOCKERFILE).upstream

build.docker: $(DOCKERFILE).upstream scm-source.json build.linux
	docker build --rm -t "$(IMAGE):$(TAG)" -f $(DOCKERFILE).upstream .

build.push: build.docker
	docker push "$(IMAGE):$(TAG)"

scm-source.json: .git
	@echo '{"url": "$(GITURL)", "revision": "$(GITHEAD)", "author": "$(USER)", "status": "$(GITSTATUS)"}' > scm-source.json
