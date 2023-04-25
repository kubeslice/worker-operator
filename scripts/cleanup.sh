#!/bin/bash
updateControllerClusterStatus() {
    kubectl patch clusters.controller.kubeslice.io $CLUSTER_NAME -n $PROJECT_NAMESPACE --token $TOKEN --certificate-authority /ca.crt --server $HUB_ENDPOINT --type=merge --subresource status --patch "{\"status\": {\"registrationStatus\": \"$1\"}}"
}

removeFinalizer() {
    kubectl get -o yaml clusters.controller.kubeslice.io $CLUSTER_NAME -n $PROJECT_NAMESPACE --token $TOKEN  --certificate-authority /ca.crt --server $HUB_ENDPOINT > ./cluster.yaml
    sed -i '/finalizers:/,/^[^ ]/ s/ *- networking.kubeslice.io\/cluster-deregister-finalizer//' cluster.yaml
    kubectl patch clusters.controller.kubeslice.io $CLUSTER_NAME -n $PROJECT_NAMESPACE --type=merge  --patch-file cluster.yaml --token $TOKEN  --certificate-authority /ca.crt --server $HUB_ENDPOINT
}

deleteKubeSliceCRDs() {
    kubectl delete crd serviceexports.networking.kubeslice.io --ignore-not-found
    kubectl delete crd serviceimports.networking.kubeslice.io --ignore-not-found
    kubectl delete crd slices.networking.kubeslice.io --ignore-not-found
    kubectl delete crd slicegateways.networking.kubeslice.io --ignore-not-found
    kubectl delete crd slicenodeaffinities.networking.kubeslice.io --ignore-not-found
    kubectl delete crd sliceresourcequotas.networking.kubeslice.io --ignore-not-found
    kubectl delete crd slicerolebindings.networking.kubeslice.io --ignore-not-found
}

deleteNamespace(){
    kubectl delete namespace kubeslice-system
}

SECRET=kubeslice-hub
TOKEN=$(kubectl get secret ${SECRET} -o json | jq -Mr '.data.token' | base64 -d)
kubectl get secret ${SECRET} -o json | jq -Mr '.data["ca.crt"]' | base64 -d > /ca.crt
# get the worker release name
workerRelease=$(helm list --output json -n kubeslice-system | jq -r '.[] | select(.chart | startswith("kubeslice-worker-")).name')
echo workerRelease $workerRelease
if helm uninstall $workerRelease --namespace kubeslice-system
then
    # delete crds
    deleteKubeSliceCRDs
    # set registration status
    updateControllerClusterStatus "Deregistered"
    # pausing for 5 sec for controller to raise events
    sleep 5
    # remove worker finalizer from cluster object
    removeFinalizer
    # delete namespace
    deleteNamespace
    # set the registration status
else
    if helm uninstall $workerRelease --namespace kubeslice-system --no-hooks
    then 
        # deleting nsm mutatingwebhookconfig
        WH=$(kubectl get pods -l app=admission-webhook-k8s  -o jsonpath='{.items[*].metadata.name}')
        kubectl delete mutatingwebhookconfiguration --ignore-not-found ${WH}

        # removing finalizers from spire and kubeslice-system namespaces
        NAMESPACES=(spire kubeslice-system)
        for ns in ${NAMESPACES[@]}
        do
            kubectl get ns $ns -o name  
            if [[ $? -eq 1 ]]; then
                echo "$ns namespace was deleted successfully"
                continue
            fi
            echo "finding and removing spiffeids in namespace $ns ..."
            for item in $(kubectl get spiffeid.spiffeid.spiffe.io -n $ns -o name); do
            echo "removing item $item"
            kubectl patch $item -p '{"metadata":{"finalizers":null}}' --type=merge -n $ns
            kubectl delete $item --ignore-not-found -n $ns
            done
        done
        # delete crds
        deleteKubeSliceCRDs
        # set the registration status
        updateControllerClusterStatus "Deregistered"
        # pausing for 5 sec for controller to raise events
        sleep 5
        # remove worker finalizer from cluster object
        removeFinalizer
        # delete namespace
        deleteNamespace
    else 
        updateControllerClusterStatus "DeregisterFailed"
    fi
fi