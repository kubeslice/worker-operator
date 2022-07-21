#!/bin/bash

# Create controller kind cluster if not present
if [ ! $(kind get clusters | grep controller) ];then
  kind create cluster --name controller --config .github/workflows/scripts/cluster.yaml --image kindest/node:v1.22.7
  ip=$(docker inspect controller-control-plane | jq -r '.[0].NetworkSettings.Networks.kind.IPAddress') 

  # Replace loopback IP with docker ip
  kind get kubeconfig --name controller | sed "s/127.0.0.1.*/$ip:6443/g" > /home/runner/.kube/kind1.yaml
fi

# Create worker1 kind cluster if not present
# Create worker1 kind cluster if not present
if [ ! $(kind get clusters | grep worker) ];then
  kind create cluster --name worker --config .github/workflows/scripts/cluster.yaml --image kindest/node:v1.22.7
  ip=$(docker inspect worker-control-plane | jq -r '.[0].NetworkSettings.Networks.kind.IPAddress')

  # Replace loopback IP with docker ip
  kind get kubeconfig --name worker | sed "s/127.0.0.1.*/$ip:6443/g" > /home/runner/.kube/kind2.yaml
fi

KUBECONFIG=/home/runner/.kube/kind1.yaml:/home/runner/.kube/kind2.yaml kubectl config view --raw  > /home/runner/.kube/kind.yaml

if [ ! -f profile/kind.yaml ];then
  # Provide correct IP in kind profile, since worker operator cannot detect internal IP as nodeIp
  ip1=$(docker inspect controller-control-plane | jq -r '.[0].NetworkSettings.Networks.kind.IPAddress')

  ip2=$(docker inspect worker1-control-plane | jq -r '.[0].NetworkSettings.Networks.kind.IPAddress')

  cat > profile/kind.yaml << EOF
Kubeconfig: kind.yaml
ControllerCluster:
  Context: kind-controller
WorkerClusters:
- Context: kind-controller
  NodeIP: ${ip1}
- Context: kind-worker1
  NodeIP: ${ip2}
WorkerChartOptions:
  SetStrValues:
    "operator.image": "worker-operator"
    "operator.tag": "e2e-latest"
TestSuitesEnabled:
  HubSuite: true
  WorkerSuite: true
EOF

fi
