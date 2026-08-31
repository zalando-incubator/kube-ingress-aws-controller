.PHONY: clean check build.local build.linux build.osx build.docker build.push

BINARY        ?= kube-ingress-aws-controller
VERSION       ?= $(shell git describe --tags --always --dirty)
IMAGE         ?= ghcr.io/zalando-incubator/$(BINARY)
TAG           ?= $(VERSION)
SOURCES       = $(shell find . -name '*.go')
DOCKERFILE    ?= Dockerfile
GOPKGS        = $(shell go list ./...)
BUILD_FLAGS   ?= -v
LDFLAGS       ?= -X main.version=$(VERSION) -X main.buildstamp=$(shell date -u '+%Y-%m-%d_%I:%M:%S%p') -X main.githash=$(shell git rev-parse HEAD) -w -s
CF_SCHEMA     ?= 206.0.0 # Run `CF_SCHEMA=latest make recreate.schema` to get the latest schema version


default: build.local

clean: ## cleans the binary
	rm -rf build
	rm -rf profile.cov

.PHONY: deps
deps: ## install dependencies to run everything
	go env
	@go install honnef.co/go/tools/cmd/staticcheck@latest
	@go install golang.org/x/vuln/cmd/govulncheck@latest
	@go install github.com/google/osv-scanner/cmd/osv-scanner@v1
	@go install github.com/google/capslock/cmd/capslock@latest
	@go install github.com/mattn/goveralls@latest

.PHONY: lint
lint: vet staticcheck ## run all linters

.PHONY: check-security
check-security: govulncheck osv-scanner capslock ## run all security checker

test: ## runs go test
	go test -v -race -coverprofile=profile.cov -cover $(GOPKGS)
	grep -v \
		-e github.com/zalando-incubator/kube-ingress-aws-controller/certs/fake/ \
		-e github.com/zalando-incubator/kube-ingress-aws-controller/aws/fake/ \
		-e github.com/zalando-incubator/kube-ingress-aws-controller/internal/aws/cloudformation/ \
		-e github.com/zalando-incubator/kube-ingress-aws-controller/internal/aws/mock/ \
		-e github.com/zalando-incubator/kube-ingress-aws-controller/internal/kubernetes/mock/ \
		profile.cov > profile.cov.tmp
	mv profile.cov.tmp profile.cov

.PHONY: vet
vet: $(SOURCES) ## run Go vet (reliable) see https://pkg.go.dev/cmd/vet
	go vet ./...

.PHONY: staticcheck
# -ST1000 generated files
# -ST1003 wrong naming convention Api vs API, Id vs ID
# -ST1020 too many wrong comments on exported functions to fix right away
staticcheck: $(SOURCES) ## run staticcheck (reliable) see also https://staticcheck.dev/docs/checks/
	staticcheck -checks "all,-ST1000,-ST1003,-ST1020" ./...

fmt: ## formats all go files
	go fmt $(GOPKGS)

.PHONY: govulncheck
govulncheck: $(SOURCES) ## run govulncheck (reliable) see https://go.dev/blog/govulncheck
	govulncheck ./...

.PHONY: capslock
capslock: $(SOURCES) ## run capslock
	capslock -output=v -packages=./...

.PHONY: osv-scanner
osv-scanner: $(SOURCES) ## run osv-scanner (reliable) see https://osv.dev/
	osv-scanner -r ./

.PHONY: build.local
build.local: build/$(BINARY) ## builds a local binary in build directory

build.linux: build/linux/$(BINARY) ## builds a binary for linux/amd64 in build directory

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

build.docker: build.linux ## builds docker image
	docker build --rm -t "$(IMAGE):$(TAG)" -f $(DOCKERFILE) --build-arg TARGETARCH= .

build.push: build.docker ## pushes docker image to registry
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

recreate.ca: recreate.cnf ## recreates a signed local test certificate
	openssl req -config kubernetes/testdata/test.cnf -new -x509 -sha256 -nodes -keyout kubernetes/testdata/key.pem -days $$((10*365)) -out kubernetes/testdata/ca.crt -subj "/"
	cp kubernetes/testdata/ca.crt kubernetes/testdata/cert.pem

export TEST_CNF
recreate.cnf:
	@echo "$$TEST_CNF" > kubernetes/testdata/test.cnf

recreate.schema: internal/aws/cloudformation/scraper/aws_schema_test.go ## recreate AWS CloudFormation schema
	CF_SCHEMA=$(CF_SCHEMA) go test ./internal/aws/cloudformation/scraper -run=TestSchema -tags=scraper -v
	go fmt ./internal/aws/cloudformation/schema.go

help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[.a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
