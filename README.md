# kubeslice-operator

TODO: Add description

## Getting secrets from hub cluster

The following command will fetch the relevant secrets from hub cluster
and copy them to `secrets` folder. It will also output them so that we
can use it to populate helm chart values.

```
deploy/hub_secret.sh [hub_cluster_context] [tenant_namespace] [spoke_cluster_name]

eg:
deploy/hub_secret.sh gke_avesha-dev_us-east1-c_xxxx hub-avesha-tenant-cisco my-awesome-cluster
```

## Build and push docker images

Adjust `VERSION` variable in the Makefile to change the docker tag to be built.
Image is set as `nexus.dev.aveshalabs.io/kubeslice-operator:$(VERSION)` in the makefile. Change this if required

```
make docker-build
make docker-push
```

## Deploying in a cluster

Create chart values file in `deploy/kubeslice-operator/values/yourvaluesfile.yaml`.
Refer to `deploy/kubeslice-operator/values/values.yaml` on how to adjust this.

```
make chart-deploy VALUESFILE=yourvaluesfile.yaml
```

## Running locally

It is possible to run the operator locally while the remaining
components (netops, dns, router etc) are deployed in the cluster.

Install kubeslice helm chart.
Create values file in `deploy/kubeslice-operator/values/yourvaluesfile.yaml`

```
make chart-deploy VALUESFILE=yourvaluesfile.yaml
```

Scale down the operator deployment in the cluster to zero, so that we
can run the same locally

```
k scale deploy kubeslice-operator --replicas=0
```

Copy the env.sample to `.env` and make changes as required

Get the serviceaccount token and ca from hub cluster (after base64
decode) and copy them into files under `secrets` folder in this repo.

* HUB_PROJECT_NAMESPACE : namespace for the tenant in hub cluster
* HUB_HOST_ENDPOINT: get hub api endpoint by running `k cluster-info` against the hub cluster
* HUB_TOKEN_FILE : file path where hub token is kept
* HUB_CA_FILE : file path where hub ca file is kept
* ENABLE_WEBHOOKS : set to false as webhooks doesn't work locally (TODO: need to think about using telepresence later)

You can add more env variables to override defaults as needed

```
source .env
make run
```

## Developing webhooks locally

it is possible to run the operator locally and forward the webhook
requests from within the cluster to your local instance.

Copy webhook tls key and tls cert under `secrets/webhook` folder

Use the script `deploy/webhook-secret.sh` to automatically fetch webhook secrets from curent cluster and copy it to the folder.

```
deploy/webhook-secret.sh
```

```
❯ tree secrets
secrets
├── ca.crt
├── token
└── webhook
    ├── tls.crt
    └── tls.key

1 directory, 4 files
```

Adjust `.env` values

```
export ENABLE_WEBHOOKS=true
export WEBHOOK_CERTS_DIR=/home/jayadeep/workspace/work/avesha/mesh/repos/kubeslice-operator/secrets/webhook
```

Use [Telepresence](https://www.telepresence.io/) to intercept traffic into your manager pod in the
cluster and forward it locally

```
telepresence intercept kubeslice-operator -p 9443
```

Make sure an instance of operator is running in the cluster at this
time.

Now we can start the operator locally and test the webhooks

```
source .env
make run
```

When you create the corresponding kubernetes object in the cluster, the
webhook request will be forwarded into your local cluster.

To stop telepresence,

```
telepresence uninstall --everything
```
