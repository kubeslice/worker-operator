#!/bin/bash


usage(){
  cat << EOF
Fetch the webhook token and key into secrets/webhook folder for local development
Make sure the current context is pointing to the right cluster
usage: webhook_secret.sh

Dependencies:
- kubectl

EOF
}

  

echo "Getting webhook secrets from $(kubectl config current-context) and copying into secrets/webhook folder"

read -n 1 -p "Continue? (y/N) : " p
echo

if [ "$p" != "y" ];then
  exit 1
fi


mkdir -p secrets/webhook
kubectl get secret kubeslice-admission-webhook-certs -o jsonpath="{.data.tls\.key}" | base64 -d > secrets/webhook/tls.key
kubectl get secret kubeslice-admission-webhook-certs -o jsonpath="{.data.tls\.crt}" | base64 -d > secrets/webhook/tls.crt

echo

echo "Copied to secrets/webhook folder:"
tree secrets
