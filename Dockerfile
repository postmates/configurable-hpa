# Build the manager binary
FROM golang:1.11.1-alpine3.8 as builder

# Copy in the go src
WORKDIR /go/src/github.com/postmates/configurable-hpa
COPY pkg/    pkg/
COPY cmd/    cmd/
COPY vendor/ vendor/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager github.com/postmates/configurable-hpa/cmd/manager

# Copy the controller-manager into a thin image
FROM alpine:3.8
WORKDIR /root/
COPY --from=builder /go/src/github.com/postmates/configurable-hpa/manager .
ENTRYPOINT ["./manager"]
