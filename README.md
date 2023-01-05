# Kubeslice Worker Operator


The Kubeslice Worker Operator, also known as Slice Operator manages the life-cycle of KubeSlice worker cluster related [custom resource definitions (CRDs)](https://kubernetes.io/docs/tasks/access-kubernetes-api/extend-api-custom-resource-definitions).
The `kubeslice-worker` operator uses Kubebuilder, a framework for building Kubernetes APIs using CRDS.

## Getting Started

Please refer to our documentation on:
- [Installing KubeSlice on cloud clusters](https://kubeslice.io/documentation/open-source/0.5.0/getting-started-with-cloud-clusters/installing-kubeslice/installing-the-kubeslice-controller)
- [Installing KubeSlice on kind clusters](https://kubeslice.io/documentation/open-source/0.5.0/tutorials/kind-install-kubeslice-controller)

## Installing `kubeslice-worker` on a Kind Cluster

### Prerequisites

Before you begin,make sure the following prerequisites are met:
* Docker is installed and running on your local machine.
* A running [`kind`](https://kind.sigs.k8s.io/).
* [`kubectl`](https://kubernetes.io/docs/tasks/tools/) is installed and configured.
* You have prepared the environment to install [`kubeslice-controller`](https://github.com/kubeslice/kubeslice-controller) on the controller cluster and [`worker-operator`](https://github.com/kubeslice/worker-operator) on the worker cluster. For more information, see [Prerequisites](https://kubeslice.io/documentation/open-source/0.5.0/getting-started-with-cloud-clusters/prerequisites/).

### Local Build and Update

#### Latest Docker Hub Image

```console
docker pull aveshasystems/worker-operator:latest
```

### Setting up Your Helm Repo

If you have not added avesha helm repo yet, add it.

```console
helm repo add avesha https://kubeslice.github.io/charts/
```

Upgrade the avesha helm repo.

```console
helm repo update
```

### Getting Secrets from Controller Cluster (if it's not already done)

The following command will fetch the relevant secrets from the controller cluster
and copy them to the `secrets` folder. Additionally, it will also output the secrets so that we
can use them to populate helm chart values.

```console
deploy/controller_secret.sh [controller_cluster_context] [project_namespace] [worker_cluster_name]

```
Example

```
deploy/controller_secret.sh gke_avesha-dev_us-east1-c_xxxx kubeslice-cisco my-awesome-cluster
```

### Build Docker Images

1. Clone the latest version of worker-operator from  the `master` branch.

```bash
git clone https://github.com/kubeslice/worker-operator.git
cd worker-operator
```

2. Adjust the `VERSION` variable in the Makefile to change the docker tag to be built.
Image is set as `docker.io/aveshasystems/worker-operator:$(VERSION)` in the Makefile. Change this if required.

```console
make docker-build
```


### Running the Local Image on Kind Cluster

1. You can load the operator on your Kind cluster with the below command.

```console
kind load docker-image <my-custom-image>:<unique-tag> --name <cluster-name>
```

example:

```console
kind load docker-image aveshasystems/worker-operator:1.2.1 --name kind
```

2. Check the loaded image in the cluster. Modify node name if required.

```console
docker exec -it <node-name> crictl images
```

example:

```console
docker exec -it kind-control-plane crictl images
```

### Deploying in a Cluster

Create the chart values file, `yourvaluesfile.yaml`.
Refer to [values.yaml](https://github.com/kubeslice/charts/blob/master/charts/kubeslice-worker/values.yaml) to create `yourvaluesfile.yaml` and update the operator image subsection to use the local image.

From the sample: 

```
operator:
  image: docker.io/aveshasystems/worker-operator
  tag: 0.2.3
```

Change it to: 

```
operator:
  image: <my-custom-image> 
  tag: <unique-tag>
````

Deploy the Updated Chart

```console
make chart-deploy VALUESFILE=yourvaluesfile.yaml
```

### Verify the Installation

Verify the installation by checking the status of pods belonging to the `kubeslice-system` namespace.

```console
kubectl get pods -n kubeslice-system
```

Example output 

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

### Uninstalling the Worker Operator

For more information, see [deregistering the worker cluster](https://kubeslice.io/documentation/open-source/0.5.0/getting-started-with-cloud-clusters/uninstalling-kubeslice/deregistering-the-worker-cluster).

```console
helm uninstall kubeslice-worker -n kubeslice-system
 ```

## License

Apache License 2.0
 
