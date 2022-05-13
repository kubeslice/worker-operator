# kubeslice-worker operator

[![Docker Image Size](https://img.shields.io/docker/image-size/aveshasystems/worker-operator)
[![Docker Image Version](https://img.shields.io/docker/v/aveshasystems/worker-operator?sort=date)

The kubeslice-worker operator manages the life-cycle of KubeSlice worker cluster related [custom resource definitions (CRDs)](https://kubernetes.io/docs/tasks/access-kubernetes-api/extend-api-custom-resource-definitions).
kubeslice-worker operator uses Kubebuilder, a framework for building Kubernetes APIs using CRDS.

## Getting Started

[TBD: Add getting started link] 
It is strongly recommended to use a released version.

## Installing `kubeslice-worker` in kind cluster

### Prerequisites

* Docker installed and running in your local machine
* A running [`kind`](https://kind.sigs.k8s.io/) 
* [`kubectl`](https://kubernetes.io/docs/tasks/tools/) installed and configured
* Follow the getting started from above, to install [`kubeslice-controller`](https://github.com/kubeslice/kubeslice-controller) and [`worker-operator`](https://github.com/kubeslice/worker-operator)

### Local build and update

#### Latest docker image
[TBD link to docker hub]

### Setting up your helm repo

If you have not added avesha helm repo yet, add it

```console
helm repo add avesha https://kubeslice.github.io/charts/
```

upgrade the avesha helm repo

```console
helm repo update
```

### Getting secrets from controller cluster, if not already done

The following command will fetch the relevant secrets from controller cluster
and copy them to `secrets` folder. It will also output them so that we
can use it to populate helm chart values.

```console
deploy/controller_secret.sh [controller_cluster_context] [project_namespace] [worker_cluster_name]

```
eg:

```
deploy/controller_secret.sh gke_avesha-dev_us-east1-c_xxxx controller-avesha-tenant-cisco my-awesome-cluster
```
### Build docker images

1. Clone the latest version of worker-operator from  the `master` branch.

```bash
git clone https://github.com/kubeslice/worker-operator.git
cd worker-operator
```

2. Adjust `VERSION` variable in the Makefile to change the docker tag to be built.
Image is set as `docker.io/aveshasystems/worker-operator:$(VERSION)` in the Makefile. Change this if required

```console
make docker-build
```


### Running local image on Kind

You can load the operator on your Kind cluster with the below command

```console
kind load docker-image <my-custom-image>:<unique-tag> --name <cluster-name>
```

### Deploying in a cluster

Create chart values file `yourvaluesfile.yaml`.
Refer to [values.yaml](https://raw.githubusercontent.com/kubeslice/charts/master/kubeslice-worker/values.yaml?token=GHSAT0AAAAAABTXBAR34JSRCDHTKG4KFGNIYT5AZ4Q) on how to adjust this and update the operator image the local image.

From the sample , 

```
operator:
  image: docker.io/aveshasystems/worker-operator
  tag: 0.2.3
```

change it to , 

```
operator:
  image: <my-custom-image> 
  tag: <unique-tag>
````

Deploy the updated chart

```console
make chart-deploy VALUESFILE=yourvaluesfile.yaml
```

### Verify the operator is running

```console
kubectl get pods -n kubeslice-system
```

Sample output to expect

```
NAME                                     READY   STATUS    RESTARTS   AGE
jaeger-65c6b7f5dd-frxtx                  1/1     Running   0          49s
kubeslice-netop-g4hqd                    1/1     Running   0          49s
kubeslice-operator-6844b47cf8-c8lv2      2/2     Running   0          48s
mesh-dns-65fd8585ff-nlp5h                1/1     Running   0          48s
nsm-admission-webhook-7b848ffc4b-dhn96   1/1     Running   0          48s
nsm-kernel-forwarder-fd74h               1/1     Running   0          49s
nsm-kernel-forwarder-vvrp6               1/1     Running   0          49s
nsmgr-62kdk                              3/3     Running   0          48s
nsmgr-7dh2w                              3/3     Running   0          48s
prefix-service-76bd89c44f-2p6dw          1/1     Running   0          48s
```

## License

Apache License 2.0
