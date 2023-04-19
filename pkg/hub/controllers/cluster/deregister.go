package cluster

import (
	"context"

	"github.com/kubeslice/worker-operator/pkg/logger"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	serviceAccountName         = "cluster-deregister-sa"
	clusterRoleName            = "cluster-deregister-role"
	clusterRoleBindingName     = "cluster-deregister-rb"
	clusterDeregisterConfigMap = "cluster-deregister-cm"
	deregisterJobName          = "cluster-deregisteration-job"
)

func (r *Reconciler) CreateDeregisterJob(ctx context.Context) error {
	log := logger.FromContext(ctx).WithName("cluster-deregister")
	// 1. create service account
	sa := constructServiceAccount()
	if err := r.Client.Create(ctx, sa, &client.CreateOptions{}); err != nil {
		log.Error(err, "unable to create service account")
		return err
	}
	if err := r.Client.Create(ctx, constructClusterRole(), &client.CreateOptions{}); err != nil {
		log.Error(err, "unable to create cluster role")
		return err
	}
	// 2. create cluster rolebinding
	rolebinding := constructClusterRoleBinding()
	if err := r.Client.Create(ctx, rolebinding, &client.CreateOptions{}); err != nil {
		log.Error(err, "unable to create cluster rolebinding")
		return err
	}
	// create configmap
	configMap := constructConfigMap()
	if err := r.Client.Create(ctx, configMap, &client.CreateOptions{}); err != nil {
		log.Error(err, "unable to create cluster configmap")
		return err
	}
	// create job
	// Todo notify controller cluster deregistration in progress
	// Todo raise events
	// delete service account/cluster role/cluster rb after everything
	job := constructJobForClusterDeregister()
	if err := r.Client.Create(ctx, job, &client.CreateOptions{}); err != nil {
		log.Error(err, "unable to create job for cluster deregister")
		return err
	}
	return nil
}

func constructServiceAccount() *corev1.ServiceAccount {
	svcAcc := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: ControlPlaneNamespace,
		},
	}
	return svcAcc
}

// Todo work on policies
func constructClusterRole() *rbacv1.ClusterRole {
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterRoleBindingName,
			Namespace: ControlPlaneNamespace,
		},
	}
	return clusterRole
}

func constructClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	clusterRole := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterRoleBindingName,
			Namespace: ControlPlaneNamespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      serviceAccountName,
			Namespace: ControlPlaneNamespace,
		}},
	}
	return clusterRole
}

func constructConfigMap() *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterDeregisterConfigMap,
			Namespace: ControlPlaneNamespace,
		},
		Data: map[string]string{
			"kubeslice-cleanup.sh": `
				#!/bin/bash
				updateControllerCluster() {
				curl -i --location --request PATCH $HubEndpoint/apis/controller.kubeslice.io/v1alpha1/namespaces/$ProjectNamespace/clusters/$ClusterName --header 'Content-Type: application/merge-patch+json' --header "Authorization: Bearer $TOKEN" --cacert /ca.crt --data-raw '{
					"spec": {
						"networkInterface": "'"$1"'"
					}
				}'
				}
				
				deleteKubeSliceCRDs() {
				kubectl delete crd serviceexports.networking.kubeslice.io
				kubectl delete crd serviceimports.networking.kubeslice.io
				kubectl delete crd slice.networking.kubeslice.io
				kubectl delete crd slicegateways.networking.kubeslice.io
				kubectl delete crd slicenodeaffinities.networking.kubeslice.io
				kubectl delete crd sliceresourcequotas.networking.kubeslice.io
				kubectl delete crd slicerolebindings.networking.kubeslice.io
				}
		
				deleteNamespace(){
				kubectl delete namespace kubeslice-system
				}
		
				SECRET=kubeslice-hub
				TOKEN=$(kubectl get secret ${SECRET} -o json | jq -Mr '.data.token' | base64 -d)
				kubectl get secret ${SECRET} -o json | jq -Mr '.data["ca.crt"]' | base64 -d > /ca.crt
				if helm uninstall kubeslice-worker --namespace kubeslice-system --dry-run
				then
					updateControllerCluster "DeregisterSuccess"
					# delete crds
					deleteKubeSliceCRDs
					# delete namespace
					deleteNamespace
				else
					if helm uninstall kubeslice-worker --namespace kubeslice-system --no-hooks --dry-run
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
							updateControllerCluster "DeregisterSuccess"
							# delete crds
							deleteKubeSliceCRDs
							# delete namespace
							deleteNamespace
					else 
						updateControllerCluster "DeregisterFailed"
					fi
				fi`,
		}}
	return cm
}

func constructJobForClusterDeregister() *batchv1.Job {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deregisterJobName,
			Namespace: ControlPlaneNamespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: deregisterJobName,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: serviceAccountName,
					Containers: []corev1.Container{{
						Name:    serviceAccountName,
						Image:   "dtzar/helm-kubectl",
						Command: []string{"/bin/bash", "/tmp/kubeslice-cleanup.sh"},
						Env: []corev1.EnvVar{
							corev1.EnvVar{Name: "ProjectNamespace", Value: ProjectNamespace},
							corev1.EnvVar{Name: "HubEndpoint", Value: HubEndpoint},
							corev1.EnvVar{Name: "ClusterName", Value: ClusterName},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								MountPath: "/tmp",
								Name:      "script-data",
							},
						}},
					},
					Volumes: []corev1.Volume{{
						Name: "script-data",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: clusterDeregisterConfigMap},
							},
						}},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}
	return job
}
