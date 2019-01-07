# Build the manager binary
FROM golang:1.10.3 as builder

# Copy in the go src
WORKDIR /go/src/github.com/caicloud/kube-er-scheduler
COPY pkg/     pkg/
COPY cmd/     cmd/
COPY vendor/  vendor/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o scheduler-extender github.com/caicloud/kube-er-scheduler/cmd/scheduler/

# Copy the scheduler-extender into a busybox image
FROM busybox:latest
WORKDIR /root/
COPY --from=builder /go/src/github.com/caicloud/kube-er-scheduler .
# --v=4 --stderrthreshold=info
ENTRYPOINT ["./scheduler-extender"]
