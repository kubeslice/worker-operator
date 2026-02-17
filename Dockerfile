# syntax=docker/dockerfile:1.4
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

# Build the manager binary
FROM golang:1.25-alpine AS builder

WORKDIR /workspace

# Multi-arch build args (injected by buildx when using --platform)
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG TARGETVARIANT
# TARGETVARIANT is set for arm/v7 (e.g. arm32); empty for amd64/arm64

# Copy module manifests and vendor first (better layer cache)
COPY go.mod go.sum ./
COPY vendor/ vendor/

# Copy source
COPY main.go ./
COPY api/ api/
COPY controllers/ controllers/
COPY pkg/ pkg/
COPY events/ events/

# Build with cache mount for faster rebuilds; -ldflags -s -w reduces binary size
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GO111MODULE=on \
    go build -mod=vendor -trimpath -ldflags="-s -w" -o manager main.go

# Final image: distroless static (multi-arch manifest)
FROM gcr.io/distroless/static-debian12:nonroot
LABEL maintainer="Avesha Systems"

WORKDIR /
COPY --from=builder /workspace/manager .

# Copy manifest files for istio gateways deployment
COPY files files
COPY scripts scripts

ENV MANIFEST_PATH="/files/manifests"
ENV SCRIPT_PATH="/scripts"

USER 65532:65532
ENTRYPOINT ["/manager"]

