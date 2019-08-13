# Build the manager binary
FROM golang:1.10.3 as builder

# Copy in the go src
WORKDIR /go/src/github.com/caicloud/kube-extended-resource/extended-resource-controller
COPY pkg/     pkg/
COPY cmd/     cmd/
COPY vendor/  vendor/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o extended-resource-controller github.com/caicloud/kube-extended-resource/extended-resource-controller/cmd/controller/

# Copy the extended-resource-controller into a empty image
FROM busybox:latest
WORKDIR /root/
COPY --from=builder /go/src/github.com/caicloud/kube-extended-resource/extended-resource-controller .
ENTRYPOINT ["./extended-resource-controller"]
