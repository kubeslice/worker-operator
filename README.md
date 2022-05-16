# kubeslice-worker operator


The kubeslice-worker operator manages the life-cycle of KubeSlice worker cluster related [custom resource definitions (CRDs)](https://kubernetes.io/docs/tasks/access-kubernetes-api/extend-api-custom-resource-definitions).
kubeslice-worker operator uses Kubebuilder, a framework for building Kubernetes APIs using CRDS.

## Getting Started
It is strongly recommended to use a released version.

For information on installing KubeSlice on kind clusters, see [getting started with kind clusters](https://docs.avesha.io/opensource/getting-started-with-kind-clusters) or try out the example script in [kind-based example](https://github.com/kubeslice/examples/tree/master/kind).

For information on installing KubeSlice on cloud clusters, see [getting started with cloud clusters](https://docs.avesha.io/opensource/getting-started-with-cloud-clusters). 


## Installing `kubeslice-worker` in Kind Cluster

### Prerequisites

* Docker installed and running in your local machine
* A running [`kind`](https://kind.sigs.k8s.io/) 
* [`kubectl`](https://kubernetes.io/docs/tasks/tools/) installed and configured
* Follow the getting started from above, to install [`kubeslice-controller`](https://github.com/kubeslice/kubeslice-controller) and [`worker-operator`](https://github.com/kubeslice/worker-operator)

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

The following command will fetch the relevant secrets from controller cluster
and copy them to `secrets` folder. It will also output them so that we
can use it to populate helm chart values.

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

2. Adjust `VERSION` variable in the Makefile to change the docker tag to be built.
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

Create chart values file `yourvaluesfile.yaml`.
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

### Verify if the Operator is Running

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

### Uninstalling the worker-operator

For more information, see [deregistering the worker cluster](https://docs.avesha.io/opensource/de-registering-the-worker-cluster).

```console
helm uninstall kubeslice-worker -n kubeslice-system
 ```

## License

Apache License 2.0
