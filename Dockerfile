# Build the manager binary
FROM golang:1.14 as cache

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download
# Seed controller-gen
COPY Makefile.ci Makefile

FROM cache as base
# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY internal/ internal/
COPY redskyapi/ redskyapi/

# Build
FROM base as builder
ARG LDFLAGS=""
RUN CGO_ENABLED=0 GO111MODULE=on go build -ldflags "${LDFLAGS}" -a -o manager main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot as controller
WORKDIR /
COPY --from=builder /workspace/manager .
USER nonroot:nonroot
ENTRYPOINT ["/manager"]

#### Test
FROM base as test
RUN make test
