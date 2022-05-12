__='
   Copyright (c) 2022 Avesha, Inc. All rights reserved.
   
   SPDX-License-Identifier: Apache-2.0
   
   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at
   
   http://www.apache.org/licenses/LICENSE-2.0
   
   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
'

#!/bin/bash


usage(){
  cat << EOF
Fetch the hub cluster details and credentials, print and save them to \`secrets\` folder.
usage: $0 [controller_cluster_context] [project_namespace] [worker_cluster_name]

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

read -p "Continue? (y/N) : " p
echo

if [ "$p" != "y" ];then
  exit 1
fi


secret=$(kubectl get serviceaccount --context "$1" -n "$2" "kubeslice-rbac-worker-$3" -o jsonpath="{.secrets[0].name}")


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
