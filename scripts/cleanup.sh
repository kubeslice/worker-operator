#!/bin/bash

SECRET=kubeslice-hub
TOKEN=$(kubectl get secret ${SECRET} -o json | jq -Mr '.data.token' | base64 -d)
kubectl get secret ${SECRET} -o json | jq -Mr '.data["ca.crt"]' | base64 -d > /ca.crt
# get the worker release name
workerRelease=$(helm list --output json -n kubeslice-system | jq -r '.[] | select(.chart | startswith("kubeslice-worker-")).name')
echo "🏃 Running helm uninstall for the release $workerRelease" 

update_controller_cluster_status() {
    echo "🔧 Updating registration status"
    kubectl patch clusters.controller.kubeslice.io $CLUSTER_NAME -n $PROJECT_NAMESPACE --token $TOKEN --certificate-authority /ca.crt --server $HUB_ENDPOINT --type=merge --subresource status --patch "{\"status\": {\"registrationStatus\": \"$1\"}}"
}

remove_cluster_finalizer() {
    echo "🗑 Removing cluster deregister finalizer"
    kubectl get -o yaml clusters.controller.kubeslice.io $CLUSTER_NAME -n $PROJECT_NAMESPACE --token $TOKEN  --certificate-authority /ca.crt --server $HUB_ENDPOINT > ./cluster.yaml
    sed -i '/finalizers:/,/^[^ ]/ s/ *- worker.kubeslice.io\/cluster-deregister-finalizer//' cluster.yaml
    kubectl patch clusters.controller.kubeslice.io $CLUSTER_NAME -n $PROJECT_NAMESPACE --type=merge  --patch-file cluster.yaml --token $TOKEN  --certificate-authority /ca.crt --server $HUB_ENDPOINT
}

delete_kubeSlice_CRDs() {
    for item in $(kubectl get crd | grep "networking.kubeslice.io" | awk '{print $1}'); do
    echo "🗑 Removing item $item"
    kubectl delete crd $item --ignore-not-found
    done
}

delete_worker_namespace(){
    echo "🗑 Removing kubeslice-system namespace"
    kubectl delete namespace kubeslice-system
}

helm_uninstall_succeeded(){
    helm uninstall $workerRelease --namespace kubeslice-system > /dev/null 2>&1
    return $?
}

delete_dashboard_rbac(){
    kubectl delete clusterrole kubeslice-kubernetes-dashboard
    kubectl delete clusterrolebinding kubeslice-kubernetes-dashboard
}

run_post_uninstall_cleanup() {
    # delete crds
    delete_kubeSlice_CRDs
    # set registration status
    update_controller_cluster_status "Deregistered"
    # pausing for 5 sec for controller to raise events
    sleep 5
    # remove worker finalizer from cluster object
    remove_cluster_finalizer
    echo "🎉 Deregister successful."
    delete_dashboard_rbac
    # delete namespace
    delete_worker_namespace
}

helm_uninstall_no_hook_succeeded(){
    helm uninstall $workerRelease --namespace kubeslice-system --no-hooks > /dev/null 2>&1
    return $?
}

run_forced_post_uninstall_cleanup(){
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
    delete_kubeSlice_CRDs
    # set the registration status
    update_controller_cluster_status "Deregistered"
    # pausing for 5 sec for controller to raise events
    sleep 5
    # remove worker finalizer from cluster object
    remove_cluster_finalizer
    echo "🎉 Deregister successful."
    delete_dashboard_rbac
    # delete namespace
    delete_worker_namespace
}

if helm_uninstall_succeeded; then
  run_post_uninstall_cleanup
else
    echo "❌ Failed to uninstall worker-operator retrying with --no-hooks flag"
    if helm_uninstall_no_hook_succeeded; then 
        run_forced_post_uninstall_cleanup
    else
        echo "❌ Failed to uninstall worker-operator"
        update_controller_cluster_status "DeregisterFailed"
    fi
fi