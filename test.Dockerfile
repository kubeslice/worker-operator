<<<<<<< HEAD
FROM golang:1.23.3 AS builder
=======
FROM golang:1.24.0 AS builder
>>>>>>> b6bcef51 (Fix: Update Go version in test.Dockerfile)

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
COPY events/ events/
# Copy script files
ENV SCRIPT_PATH="/scripts"
COPY scripts /scripts

# Copy manifest files for istio gateways deployment
COPY files /files
ENV MANIFEST_PATH="/files/manifests"

CMD ["make", "test"]
