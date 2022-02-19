#!/bin/bash


usage(){
  cat << EOF
Fetch the hub cluster details and credentials, print and save them to \`secrets\` folder.
usage: hub_secret.sh [hub_cluster_context] [tenant_namespace] [spoke_cluster_name]

Dependencies:
- kubectl
- jq

EOF
}

  
if [ $# -ne 3 ]; then
  usage
fi


echo "Getting secrets from hub"
echo
echo "Cluster: $1"
echo "Namespace: $2"
echo "Spoke Cluster Name: $3"

read -n 1 -p "Continue? (y/N) : " p
echo

if [ "$p" != "y" ];then
  exit 1
fi


secret=$(kubectl get serviceaccount --context "$1" -n "$2" "kube-slice-spoke-$3" -o jsonpath="{.secrets[0].name}")


echo "Serviceaccount secret: $secret"
echo "Secret json:"

echo "============================"
kubectl get secret --context "$1" -n "$2" $secret -o jsonpath="{.data}" | jq
echo "============================"

echo "Hub endpoint:"
kubectl --context $1 cluster-info | grep "control plane" | grep  "http.*" --only-matching


mkdir -p secrets
kubectl get secret --context "$1" -n "$2" $secret -o jsonpath="{.data.ca\.crt}" | base64 -d > secrets/ca.crt
kubectl get secret --context "$1" -n "$2" $secret -o jsonpath="{.data.token}" | base64 -d > secrets/token

echo

echo "Copied to secrets folder:"
tree secrets
