# syntax=docker/dockerfile:1
##########################################################
#Dockerfile
#Copyright (c) 2022 Avesha, Inc. All rights reserved.
#
#SPDX-License-Identifier: Apache-2.0
#
#Licensed under the Apache License, Version 2.0 (the "License");
#you may not use this file except in compliance with the License.
#You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
#Unless required by applicable law or agreed to in writing, software
#distributed under the License is distributed on an "AS IS" BASIS,
#WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#See the License for the specific language governing permissions and
#limitations under the License.
##########################################################
FROM --platform=$BUILDPLATFORM golang:1.24 AS builder
LABEL maintainer="Avesha Systems"
ARG TARGETOS
ARG TARGETARCH
ARG BUILDPLATFORM
WORKDIR /workspace

# Copy the Go Modules manifests first for better layer caching
COPY go.mod go.sum ./
# Copy vendor directory (required for -mod=vendor build)
COPY vendor vendor/

# Copy the go source
COPY main.go ./
COPY api/ api/
COPY controllers/ controllers/
COPY pkg/ pkg/
COPY events/ events/

# Cross-compile with optimizations and caching
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    CGO_ENABLED=0 \
    GOOS=${TARGETOS:-linux} \
    GOARCH=${TARGETARCH} \
    go build -mod=vendor -ldflags="-w -s" -trimpath -o manager main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static-debian12:nonroot
LABEL maintainer="Avesha Systems"
WORKDIR /
COPY --from=builder /workspace/manager .

# Copy manifest files for istio gateways deployment
COPY files files
ENV MANIFEST_PATH="/files/manifests"
# Copy script files
ENV SCRIPT_PATH="/scripts"
COPY scripts scripts

USER 65532:65532

ENTRYPOINT ["/manager"]

