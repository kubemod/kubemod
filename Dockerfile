# Build the manager binary
FROM golang:1.17.0 as builder

ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY core/ core/
COPY jsonpath/ jsonpath/
COPY expressions/ expressions/
COPY util/ util/
COPY app/ app/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} GO111MODULE=on go build -a -o kubemod main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM alpine:3.12.3

RUN adduser -D nonroot

WORKDIR /
COPY --from=builder /workspace/kubemod .
USER nonroot:nonroot

ENTRYPOINT ["/kubemod", "-operator", "-webapp"]
