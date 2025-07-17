.PHONY: clean check build.local build.linux build.osx build.docker build.push

BINARY        ?= kube-ingress-aws-controller
VERSION       ?= $(shell git describe --tags --always --dirty)
IMAGE         ?= registry-write.opensource.zalan.do/teapot/$(BINARY)
TAG           ?= $(VERSION)
SOURCES       = $(shell find . -name '*.go')
DOCKERFILE    ?= Dockerfile
GOPKGS        = $(shell go list ./...)
BUILD_FLAGS   ?= -v
LDFLAGS       ?= -X main.version=$(VERSION) -X main.buildstamp=$(shell date -u '+%Y-%m-%d_%I:%M:%S%p') -X main.githash=$(shell git rev-parse HEAD) -w -s


default: build.local

## clean: cleans the binary
clean:
	rm -rf build
	rm -rf profile.cov

## test: runs go test
test:
	go test -v -race -coverprofile=profile.cov -cover $(GOPKGS)
	grep -Ev 'github.com/zalando-incubator/kube-ingress-aws-controller/(certs/fake|aws/fake|aws/cloudformation)/' profile.cov > tmp.profile.cov
	mv tmp.profile.cov profile.cov

## lint: runs golangci-lint
lint:
	golangci-lint run --timeout 5m ./...

## fmt: formats all go files
fmt:
	go fmt $(GOPKGS)

## build.local: builds a local binary in build directory
build.local: build/$(BINARY)

## build.linux: builds a binary for linux/amd64 in build directory
build.linux: build/linux/$(BINARY)

build.linux.amd64: build/linux/amd64/$(BINARY)
build.linux.arm64: build/linux/arm64/$(BINARY)

build/$(BINARY): $(SOURCES)
	CGO_ENABLED=0 go build -o build/$(BINARY) $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" .

build/linux/$(BINARY): $(SOURCES)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(BUILD_FLAGS) -o build/linux/$(BINARY) -ldflags "$(LDFLAGS)" .

build/linux/amd64/$(BINARY): go.mod $(SOURCES)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(BUILD_FLAGS) -o build/linux/amd64/$(BINARY) -ldflags "$(LDFLAGS)" .

build/linux/arm64/$(BINARY): go.mod $(SOURCES)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(BUILD_FLAGS) -o build/linux/arm64/$(BINARY) -ldflags "$(LDFLAGS)" .

## build.docker: builds docker image
build.docker: build.linux
	docker build --rm -t "$(IMAGE):$(TAG)" -f $(DOCKERFILE) --build-arg TARGETARCH= .

## build.push: pushes docker image to registry
build.push: build.docker
	docker push "$(IMAGE):$(TAG)"

define TEST_CNF
[req]
default_bits       = 2048
default_md         = sha256
distinguished_name = req_distinguished_name
x509_extensions    = x509_ext
req_extensions     = v3_req
string_mask        = utf8only
[req_distinguished_name]
commonName         = Common Name (e.g. server FQDN or YOUR name)
commonName_default = /
[x509_ext]
subjectKeyIdentifier    = hash
authorityKeyIdentifier  = keyid,issuer
basicConstraints        = CA:TRUE
keyUsage                = nonRepudiation, digitalSignature, keyEncipherment
subjectAltName          = @alt_names
[v3_req]
subjectKeyIdentifier = hash
basicConstraints     = CA:TRUE
keyUsage             = nonRepudiation, digitalSignature, keyEncipherment
subjectAltName       = @alt_names
[alt_names]
DNS.1 = *.domain.name
IP.1  = 127.0.0.1
endef

## recreate.ca: recreates a signed local test certificate
recreate.ca: recreate.cnf
	openssl req -config kubernetes/testdata/test.cnf -new -x509 -sha256 -nodes -keyout kubernetes/testdata/key.pem -days $$((10*365)) -out kubernetes/testdata/ca.crt -subj "/"
	cp kubernetes/testdata/ca.crt kubernetes/testdata/cert.pem

export TEST_CNF
recreate.cnf:
	@echo "$$TEST_CNF" > kubernetes/testdata/test.cnf

## help: prints this help message
help:
	@echo "Usage: \n"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'
