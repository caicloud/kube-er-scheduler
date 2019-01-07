all: build

TAG?=v0.1.0
IMG?=scheduler-extender:${TAG}

build: clean fmt
	go build -o bin/scheduler-extender github.com/caicloud/kube-er-scheduler/cmd/scheduler/

clean:
	rm -f bin/scheduler-extender

fmt:
	go fmt ./pkg/... ./cmd/...

test: fmt
	go test ./pkg/...

docker-build:
	docker build . -t ${IMG}

docker-push:
	docker push ${IMG}

.PHONY: all build clean fmt test