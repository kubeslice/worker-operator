#!/bin/bash

HOST_IP=$(hostname -i)

# Create controller kind cluster if not present
if [ ! $(kind get clusters | grep controller) ];then
  kind create cluster --name controller --config .github/workflows/scripts/cluster.yaml --image kindest/node:v1.22.7
  ip=$(docker inspect controller-control-plane | jq -r '.[0].NetworkSettings.Networks.kind.IPAddress')

  # Replace loopback IP with docker ip
  kind get kubeconfig --name controller | sed "s/127.0.0.1.*/$HOST_IP:6443/g" > /home/runner/.kube/kind1.yaml
fi

# Create worker1 kind cluster if not present
if [ ! $(kind get clusters | grep worker1) ];then
  kind create cluster --name worker --config .github/workflows/scripts/cluster.yaml --image kindest/node:v1.22.7
  ip=$(docker inspect worker-control-plane | jq -r '.[0].NetworkSettings.Networks.kind.IPAddress')

  # Replace loopback IP with docker ip
  kind get kubeconfig --name worker | sed "s/127.0.0.1.*/$HOST_IP:7443/g" > /home/runner/.kube/kind2.yaml
fi

KUBECONFIG=/home/runner/.kube/kind1.yaml:/home/runner/.kube/kind2.yaml kubectl config view --raw  > /home/runner/.kube/kinde2e.yaml

if [ ! -f profile/kind.yaml ];then
  # Provide correct IP in kind profile, since worker operator cannot detect internal IP as nodeIp
  ip1=$(docker inspect controller-control-plane | jq -r '.[0].NetworkSettings.Networks.kind.IPAddress')

  ip2=$(docker inspect worker-control-plane | jq -r '.[0].NetworkSettings.Networks.kind.IPAddress')

pwd

  cat > kind.yaml << EOF
Kubeconfig: kinde2e.yaml
ControllerCluster:
  Context: kind-controller
WorkerClusters:
- Context: kind-controller
  NodeIP: ${HOST_IP}
- Context: kind-worker
  NodeIP: ${HOST_IP}
WorkerChartOptions:
  SetStrValues:
    "operator.image": "worker-operator"
    "operator.tag": "e2e-latest"
TestSuitesEnabled:
  HubSuite: true
  WorkerSuite: true
EOF

fi
