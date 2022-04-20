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
