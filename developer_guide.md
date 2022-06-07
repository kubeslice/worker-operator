# Development guidelines for KubeSlice Worker

The KubeSlice-worker operator manages the life cycle of KubeSlice worker cluster related [CRDs](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/).
It is strongly recommended to use a released version. Follow the instructions provided in this [guide](https://docs.avesha.io/opensource/registering-the-worker-cluster#Bookmark162).

## Building and Installing `kubeslice-worker` in a Local Kind Cluster
For more information, see [getting started with kind clusters](https://docs.avesha.io/opensource/getting-started-with-kind-clusters).

### Setting up Development Environment

* Go (version 1.17 or later) installed and configured in your machine ([Installing Go](https://go.dev/dl/))
* Docker installed and running in your local machine
* A running [`kind`](https://kind.sigs.k8s.io/)  cluster
* [`kubectl`](https://kubernetes.io/docs/tasks/tools/) installed and configured
* Follow the getting started from above, to install [kubeslice-controller](https://github.com/kubeslice/kubeslice-controller) 



### Building Docker Images

1. Clone the latest version of kubeslice-controller from  the `master` branch.

```bash
git clone https://github.com/kubeslice/worker-operator.git
cd worker-operator
```

2. Adjust image name variable `IMG` in the [`Makefile`](Makefile) to change the docker tag to be built.
   Default image is set as `IMG ?= aveshasystems/worker-operator:latest`. Modify this if required.

```bash
make docker-build
```

3. Loading kubeslice-worker Image Into Your Kind Cluster ([`link`](https://kind.sigs.k8s.io/docs/user/quick-start/#loading-an-image-into-your-cluster))
   If needed, replace `aveshasystems/worker-operator` with your locally built image name in the previous step.
   [See loading an image into your cluster](https://kind.sigs.k8s.io/docs/user/quick-start/#loading-an-image-into-your-cluster)
```bash
kind load docker-image aveshasystems/worker-operator --name kind
```

4. Check the loaded image in the cluster. Modify node name if required.
```bash
docker exec -it kind-control-plane crictl images
```
### Installation
To install:

1. Create chart values file `yourvaluesfile.yaml`.
Refer to [values.yaml](https://github.com/kubeslice/charts/blob/master/charts/kubeslice-worker/values.yaml) to create `yourvaluesfile.yaml` and update the operator image subsection to use the local image.

From the sample: 

```console
operator:
  image: docker.io/aveshasystems/worker-operator
  tag: 0.2.3
```

Change it to: 

```console
operator:
  image: <my-custom-image> 
  tag: <unique-tag>
````

2. Deploy the Updated Chart

```console
make chart-deploy VALUESFILE=yourvaluesfile.yaml
```


### Running test cases

After running the below command, you should see all test cases have passed.

```bash
make test
```

### Uninstalling the kubeslice-worker

Refer to the [uninstall guide](https://docs.avesha.io/opensource/uninstalling-kubeslice)

1. [Detach](https://docs.avesha.io/opensource/detaching-the-applications) the application from the slice.

2. [Delete](https://docs.avesha.io/opensource/deleting-the-slice) the slice.

3. On the worker cluster, undeploy the kubeslice-worker charts.

```bash
# uninstall all the resources
make chart-undeploy
```

## License

Apache License 2.0
