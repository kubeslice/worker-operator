#!/bin/bash
# Create kind clusters
echo kind create cluster --name $HUB --config kind-controller.yaml
kind create cluster --name controller --config kind-controller.yaml