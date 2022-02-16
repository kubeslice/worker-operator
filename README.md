# kubeslice-operator

TODO: Add description and basic instructions here

## Running locally

It is possible to run the operator locally while the remaining
components (netops, dns, router etc) are deploted in the cluster.

Install kubeslice helm chart

```
cd deploy/kubeslice-operator
helm install kubeslice . -n kubeslice-system
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
