.PHONY: build test lint sim run docker
VERSION ?= dev

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o vsphere-hardware-exporter ./cmd/vsphere-hardware-exporter

test:
	go test -race ./...

lint:
	golangci-lint run

# Start the vCenter simulator (user "user", password "pass") for local experiments.
sim:
	go run ./hack/vcsim

run: build
	VSPHERE_URL=https://127.0.0.1:8989 VSPHERE_USERNAME=user VSPHERE_PASSWORD=pass \
	  ./vsphere-hardware-exporter --vsphere.insecure-skip-verify

docker:
	docker build --build-arg VERSION=$(VERSION) -t vsphere-hardware-exporter:$(VERSION) .
