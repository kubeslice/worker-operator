FROM golang:1.17 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
ADD vendor vendor
COPY Makefile Makefile

# Download dependencies
RUN make envtest
RUN make controller-gen

# Copy the go source
COPY api/ api/
COPY controllers/ controllers/
COPY tests/ tests/
COPY hack/ hack/
COPY config/ config
COPY pkg/ pkg/

# Copy manifest files for istio gateways deployment
COPY files /files
ENV MANIFEST_PATH="/files/manifests"

CMD ["make", "test"]
