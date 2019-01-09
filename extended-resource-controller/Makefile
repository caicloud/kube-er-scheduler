all: build

TAG?=v0.1.0
IMG?=extended-resource-controller:${TAG}

build: clean fmt
	go build -a -o bin/extended-resource-controller github.com/caicloud/kube-extended-resource/extended-resource-controller/cmd/controller/

clean:
	rm -rf bin/extended-resource-controller

fmt:
	go fmt ./pkg/... ./cmd/...

test: fmt
	go test ./pkg/...

docker-build:
	docker build . -t ${IMG}

docker-push:
	docker push ${IMG}

.PHONY: all build clean fmt test