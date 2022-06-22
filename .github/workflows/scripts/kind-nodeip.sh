#!/bin/bash

# Get the Host IP
HOST_IP=$(hostname -i)
echo $HOST_IP

sed -i "s/<NEW_IP>/${HOST_IP}/g" .github/workflows/scripts/kind-controller.yaml
sed -i "s/<NEW_IP>/${HOST_IP}/g" .github/workflows/scripts/kind-worker-1.yaml
sed -i "s/<NEW_IP>/${HOST_IP}/g" .github/workflows/scripts/kind-worker-2.yaml