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

## Setting up your helm repo

If you have not added avesha helm repo yet, add it

```
helm repo add kubeslice https://kubeslice.github.io/charts/
```

upgrade the avesha helm repo

```
helm repo update
```

## Getting secrets from controller cluster

The following command will fetch the relevant secrets from controller cluster
and copy them to `secrets` folder. It will also output them so that we
can use it to populate helm chart values.

```
deploy/controller_secret.sh [controller_cluster_context] [project_namespace] [worker_cluster_name]

eg:
deploy/controller_secret.sh gke_avesha-dev_us-east1-c_xxxx controller-avesha-tenant-cisco my-awesome-cluster
```

## Build docker images

Adjust `VERSION` variable in the Makefile to change the docker tag to be built.
Image is set as `docker.io/aveshasystems/worker-operator:$(VERSION)` in the Makefile. Change this if required

```
make docker-build
```

## Running locally on Kind

You can run the operator on your Kind cluster with the below command

```
kind load docker-image <my-custom-image>:<unique-tag> --name <cluster-name>
```

## Deploying in a cluster

Create chart values file `yourvaluesfile.yaml`.
Refer to [values.yaml](https://raw.githubusercontent.com/kubeslice/charts/master/kubeslice-worker/values.yaml?token=GHSAT0AAAAAABTXBAR34JSRCDHTKG4KFGNIYT5AZ4Q) on how to adjust this.

```
make chart-deploy VALUESFILE=yourvaluesfile.yaml
```

## Verify the operator is running

```
kubectl get pods -n kubeslice-system
```

Sample output to expect

```
NAME                                     READY   STATUS    RESTARTS   AGE
avesha-netop-9xsrh                       1/1     Running   0          42h
jaeger-65c6b7f5dd-vkhph                  1/1     Running   0          42h
kubeslice-operator-6c658d5cbd-6bvsm      2/2     Running   0          30m
mesh-dns-65fd8585ff-qbzrk                1/1     Running   0          42h
nsm-admission-webhook-7b848ffc4b-fsr24   1/1     Running   0          42h
nsm-kernel-forwarder-lxpjv               1/1     Running   0          42h
nsm-kernel-forwarder-mz69c               1/1     Running   0          42h
nsmgr-5clbs                              3/3     Running   0          42h
nsmgr-f42t5                              3/3     Running   0          42h
prefix-service-76bd89c44f-kg4lp          1/1     Running   0          42h
```

## License

Apache License 2.0
