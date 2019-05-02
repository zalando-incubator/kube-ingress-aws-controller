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

clean:
	rm -rf build

test:
	go test -v -race -cover $(GOPKGS)

lint:
	golangci-lint run ./...

fmt:
	go fmt $(GOPKGS)

check:
	golint $(GOPKGS)
	go vet -v $(GOPKGS)

build.local: build/$(BINARY)
build.linux: build/linux/$(BINARY)
build.osx: build/osx/$(BINARY)

build/$(BINARY): $(SOURCES)
	CGO_ENABLED=0 go build -o build/$(BINARY) $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" .

build/linux/$(BINARY): $(SOURCES)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(BUILD_FLAGS) -o build/linux/$(BINARY) -ldflags "$(LDFLAGS)" .

build/osx/$(BINARY): $(SOURCES)
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(BUILD_FLAGS) -o build/osx/$(BINARY) -ldflags "$(LDFLAGS)" .

build.docker: build.linux
	docker build --rm -t "$(IMAGE):$(TAG)" -f $(DOCKERFILE) .

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

recreate.ca: recreate.cnf
	openssl req -config kubernetes/testdata/test.cnf -new -x509 -sha256 -nodes -keyout kubernetes/testdata/key.pem -days $$((10*365)) -out kubernetes/testdata/ca.crt -subj "/"
	cp kubernetes/testdata/ca.crt kubernetes/testdata/cert.pem

export TEST_CNF
recreate.cnf:
	@echo "$$TEST_CNF" > kubernetes/testdata/test.cnf
