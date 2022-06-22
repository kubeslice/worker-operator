#!/bin/bash
# Create kind controller cluster
echo kind create cluster --name controller --config .github/workflows/scripts/kind-controller.yaml
kind create cluster --name controller --config .github/workflows/scripts/kind-controller.yaml

# Create kind worker-1 cluster
echo kind create cluster --name worker-1 --config .github/workflows/scripts/kind-worker-1.yaml
kind create cluster --name worker-1 --config .github/workflows/scripts/kind-worker-1.yaml

# Create kind worker-2 cluster
echo kind create cluster --name worker-2 --config .github/workflows/scripts/kind-worker-2.yaml
kind create cluster --name worker-2 --config .github/workflows/scripts/kind-worker-2.yaml