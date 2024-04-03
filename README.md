# Kubeslice Worker Operator


The Kubeslice Worker Operator, also known as Slice Operator manages the lifecycle of KubeSlice worker cluster-related [custom resource definitions (CRDs)](https://kubernetes.io/docs/tasks/access-kubernetes-api/extend-api-custom-resource-definitions).
The `kubeslice-worker` operator uses Kubebuilder, a framework for building Kubernetes APIs using CRDS.

## Get Started
It is strongly recommended that you use a released version.

Please refer to our documentation on:
- [Get Started on KubeSlice](https://kubeslice.io/documentation/open-source/1.3.0/category/get-started)
- [Install KubeSlice](https://kubeslice.io/documentation/open-source/1.3.0/category/install-kubeslice)

## Install `kubeslice-worker` on a Kind Cluster

### Prerequisites

Before you begin, make sure the following prerequisites are met:
* Docker is installed and running on your local machine.
* A running [`kind`](https://kind.sigs.k8s.io/) cluster.
* [`kubectl`](https://kubernetes.io/docs/tasks/tools/) is installed and configured.
* You have prepared the environment to install [`kubeslice-controller`](https://github.com/kubeslice/kubeslice-controller) on the controller cluster and [`worker-operator`](https://github.com/kubeslice/worker-operator) on the worker cluster. For more information, see [Prerequisites](https://kubeslice.io/documentation/open-source/1.3.0/category/prerequisites).

### Build and Deploy a Worker Operator on a Kind Cluster

To download the latest Worker Operator docker image, click [here](https://hub.docker.com/r/aveshasystems/worker-operator).

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

### Get Secrets from the Controller Cluster (if it's not already done)

The following command will get the relevant secrets from the controller cluster
and copy them to the `secrets` folder. Additionally, it will return the secrets so that we
can use them to populate the helm chart values.

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

2. Edit the `VERSION` variable in the Makefile to change the docker tag to be built.
The image is set as `docker.io/aveshasystems/worker-operator:$(VERSION)` in the Makefile. Modify this if required.

   ```console
   make docker-build
   ```


### Run the Local Image on a Kind Cluster

1. You can load the Worker Operator on your kind cluster using the following command:

   ```console
   kind load docker-image <my-custom-image>:<unique-tag> --name <cluster-name>
   ```

   Example:

   ```console
   kind load docker-image aveshasystems/worker-operator:1.2.1 --name kind
   ```

2. Check the loaded image in the cluster. Modify the node name if required.

   ```console
   docker exec -it <node-name> crictl images
   ```

   Example:

   ```console
   docker exec -it kind-control-plane crictl images
   ```

### Deploy the Worker Operator on a Cluster

Create a chart values file called `yourvaluesfile.yaml`.
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

### Uninstall the Worker Operator

For more information, see [deregister the worker cluster](https://kubeslice.io/documentation/open-source/1.3.0/uninstall-kubeslice/#deregister-worker-clusters).

```console
helm uninstall kubeslice-worker -n kubeslice-system
 ```

## License

Apache License 2.0
 
