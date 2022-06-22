#!/bin/bash

# Get the Host IP
HOST_IP=$(hostname -i)
echo $HOST_IP

sed -i "s/<NEW_IP>/${HOST_IP}/g" controller-cluster.yaml
sed -i "s/<NEW_IP>/${HOST_IP}/g" worker-cluster-1.yaml
sed -i "s/<NEW_IP>/${HOST_IP}/g" worker-cluster-2.yaml