#!/bin/bash

# Setup kind multicluster
HUB=("controller")
SPOKES=("worker-1" "worker-2")
PREFIX="kind-"

CLUSTERS=($HUB)
CLUSTERS+=(${SPOKES[*]})

# Create kind clusters
echo kind create cluster --name $HUB --config hub-cluster.yaml
kind create cluster --name $HUB --config hub-cluster.yaml

echo Create the Worker clusters
for CLUSTER in ${SPOKES[@]}; do
    echo kind create cluster --name $CLUSTER --config spoke-cluster.yaml
    kind create cluster --name $CLUSTER --config spoke-cluster.yaml
    # Make sure the cluster context exists
    kubectl cluster-info --context $PREFIX$CLUSTER
done