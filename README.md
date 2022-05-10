# kubeslice-worker operator

The kubeslice-worker operator manages the life-cycle of KubeSlice worker cluster related [custom resource definitions (CRDs)](https://kubernetes.io/docs/tasks/access-kubernetes-api/extend-api-custom-resource-definitions).
kubeslice-worker operator uses Kubebuilder, a framework for building Kubernetes APIs using CRDS.

## Getting Started

It is strongly recommended to use a released version.

## Installing `kubeslice-worker` in kind cluster

### Prerequisites

* Docker installed and running in your local machine
* A running [`kind`](https://kind.sigs.k8s.io/) or [`Docker Desktop Kubernetes`](https://docs.docker.com/desktop/kubernetes/)
  cluster
* [`kubectl`](https://kubernetes.io/docs/tasks/tools/) installed and configured
* [`kubeslice-controller`](https://github.com/kubeslice/kubeslice-controller) should be installed and setup

## Getting secrets from controller cluster

The following command will fetch the relevant secrets from controller cluster
and copy them to `secrets` folder. It will also output them so that we
can use it to populate helm chart values.

```
deploy/controller_secret.sh [controller_cluster_context] [tenant_namespace] [worker_cluster_name]

eg:
deploy/controller_secret.sh gke_avesha-dev_us-east1-c_xxxx controller-avesha-tenant-cisco my-awesome-cluster
```

## Build docker images

Adjust `VERSION` variable in the Makefile to change the docker tag to be built.
Image is set as `docker.io/aveshasystems/worker-operator:$(VERSION)` in the Makefile. Change this if required

```
make docker-build
```

## Deploying in a cluster

Create chart values file in `deploy/kubeslice-operator/values/yourvaluesfile.yaml`.
Refer to `deploy/kubeslice-operator/values/values.yaml` on how to adjust this.

```
make chart-deploy VALUESFILE=yourvaluesfile.yaml
```

## Running locally on Kind

You can run the operator on your Kind cluster with the below command

```
kind load docker-image my-custom-image:unique-tag
```

## License

Apache License 2.0
