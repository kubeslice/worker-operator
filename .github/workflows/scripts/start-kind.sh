#!/bin/bash

# Create controller kind cluster if not present
if [ ! $(kind get clusters | grep controller) ];then
  kind create cluster --name controller --config ${pwd}/cluster.yaml --image kindest/node:v1.22.7
  ip=$(docker inspect controller-control-plane | jq -r '.[0].NetworkSettings.Networks.kind.IPAddress')

  # Replace loopback IP with docker ip
  kind get kubeconfig --name controller | sed "s/127.0.0.1.*/$ip:6443/g" > /home/runner/.kube/kind1.yaml
fi

# Create worker1 kind cluster if not present
if [ ! $(kind get clusters | grep worker1) ];then
  kind create cluster --name worker --config ${pwd}/cluster.yaml --image kindest/node:v1.22.7
  ip=$(docker inspect worker1-control-plane | jq -r '.[0].NetworkSettings.Networks.kind.IPAddress')

  # Replace loopback IP with docker ip
  kind get kubeconfig --name worker | sed "s/127.0.0.1.*/$ip:6443/g" > /home/runner/.kube/kind2.yaml
fi

KUBECONFIG=/home/runner/.kube/kind1.yaml:/home/runner/.kube/kind2.yaml kubectl config view --raw  > /home/runner/.kube/kinde2e.yaml

if [ ! -f profile/kind.yaml ];then
  # Provide correct IP in kind profile, since worker operator cannot detect internal IP as nodeIp
  ip1=$(docker inspect controller-control-plane | jq -r '.[0].NetworkSettings.Networks.kind.IPAddress')

  ip2=$(docker inspect worker1-control-plane | jq -r '.[0].NetworkSettings.Networks.kind.IPAddress')

  cat > ${pwd}/profile/kind.yaml << EOF
Kubeconfig: kinde2e.yaml
ControllerCluster:
  Context: kind-controller
WorkerClusters:
- Context: kind-controller
  NodeIP: ${ip1}
- Context: kind-worker
  NodeIP: ${ip2}
TestSuitesEnabled:
  HubSuite: true
  WorkerSuite: true
EOF

fi
