#!/bin/bash
updateControllerClusterStatus() {
    echo "🔧 Updating registration status"
    kubectl patch clusters.controller.kubeslice.io $CLUSTER_NAME -n $PROJECT_NAMESPACE --token $TOKEN --certificate-authority /ca.crt --server $HUB_ENDPOINT --type=merge --subresource status --patch "{\"status\": {\"registrationStatus\": \"$1\"}}"
}

removeFinalizer() {
    echo "🗑 Removing cluster deregister finalizer"
    kubectl get -o yaml clusters.controller.kubeslice.io $CLUSTER_NAME -n $PROJECT_NAMESPACE --token $TOKEN  --certificate-authority /ca.crt --server $HUB_ENDPOINT > ./cluster.yaml
    sed -i '/finalizers:/,/^[^ ]/ s/ *- networking.kubeslice.io\/cluster-deregister-finalizer//' cluster.yaml
    kubectl patch clusters.controller.kubeslice.io $CLUSTER_NAME -n $PROJECT_NAMESPACE --type=merge  --patch-file cluster.yaml --token $TOKEN  --certificate-authority /ca.crt --server $HUB_ENDPOINT
}

deleteKubeSliceCRDs() {
    for item in $(kubectl get crd | grep "networking.kubeslice.io"); do
    echo "🗑 Removing item $item"
    kubectl delete crd $item --ignore-not-found
    done
}

deleteNamespace(){
    echo "🗑 Removing kubeslice-system namespace"
    kubectl delete namespace kubeslice-system
}

SECRET=kubeslice-hub
TOKEN=$(kubectl get secret ${SECRET} -o json | jq -Mr '.data.token' | base64 -d)
kubectl get secret ${SECRET} -o json | jq -Mr '.data["ca.crt"]' | base64 -d > /ca.crt
# get the worker release name
workerRelease=$(helm list --output json -n kubeslice-system | jq -r '.[] | select(.chart | startswith("kubeslice-worker-")).name')
echo "🏃 Running helm uninstall for the release $workerRelease" 
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
    echo "🎉 Deregister successful."
    # delete namespace
    deleteNamespace
else
    echo "❌ Failed to uninstall worker-operator retrying with --no-hooks flag"
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
                echo "✅ $ns namespace deleted successfully"
                continue
            fi
            echo "🔍 Finding spiffeids in namespace $ns ..."
            for item in $(kubectl get spiffeid.spiffeid.spiffe.io -n $ns -o name); do
            echo "🗑 Removing item $item"
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
        echo "🎉 Deregister successful."
        # delete namespace
        deleteNamespace
    else
        echo "❌ Failed to uninstall worker-operator"
        updateControllerClusterStatus "DeregisterFailed"
    fi
fi