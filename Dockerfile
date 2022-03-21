##########################################################
#Dockerfile
#
#Avesha LLC
#Sept 2020
#
#Copyright (c) Avesha LLC. 2019, 2020
# 
#Avesha Sice Operator Dockerfile
##########################################################

# Build the manager binary
FROM golang:1.17 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
ADD vendor vendor
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
#RUN echo "[url \"git@bitbucket.org:\"]\n\tinsteadOf = https://bitbucket.org/" >> /root/.gitconfig
#RUN go env -w GOPRIVATE=bitbucket.org/realtimeai && go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY internal/ internal/

# Build
RUN go env -w GOPRIVATE=bitbucket.org/realtimeai && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -mod=vendor -a -o manager main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
LABEL maintainer="Avesha Systems LLC"
WORKDIR /
COPY --from=builder /workspace/manager .

# Copy manifest files for istio gateways deployment
COPY files files
ENV MANIFEST_PATH="/files/manifests"

USER nonroot:nonroot

ENTRYPOINT ["/manager"]

