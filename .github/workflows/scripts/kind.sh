#!/bin/bash
# Create kind controller cluster
echo kind create cluster --name controller --config .github/workflows/scripts/kind-controller.yaml
kind create cluster --name controller --config .github/workflows/scripts/kind-controller.yaml

# Create kind worker cluster
echo kind create cluster --name worker --config .github/workflows/scripts/kind-worker.yaml
kind create cluster --name worker --config .github/workflows/scripts/kind-worker.yaml