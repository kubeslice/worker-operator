#!/bin/bash
# Create kind clusters
echo kind create cluster --name kind-controller --config .github/workflows/scripts/kind-controller.yaml
kind create cluster --name kind-controller --config .github/workflows/scripts/kind-controller.yaml