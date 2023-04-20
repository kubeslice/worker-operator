package cluster

import (
	"context"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	"github.com/kubeslice/worker-operator/pkg/logger"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	serviceAccountName         = "cluster-deregister-sa"
	clusterRoleName            = "cluster-deregister-role"
	clusterRoleBindingName     = "cluster-deregister-rb"
	clusterDeregisterConfigMap = "cluster-deregister-cm"
	deregisterJobName          = "cluster-deregisteration-job"
)

func (r *Reconciler) createDeregisterJob(ctx context.Context, cluster *hubv1alpha1.Cluster) error {
	log := logger.FromContext(ctx).WithName("cluster-deregister")
	// Notify controller that the deregistration process of the cluster is in progress.
	err := r.updateClusterDeregisterStatus("DeregisterInProgress", ctx, cluster)
	if err != nil {
		log.Error(err, "error updating status of deregistration on the controller")
		return err
	}
	// Steps to follow:
	// 1. Create a service account.
	// 2. Define a cluster role.
	// 3. Bind the service account with the cluster role.
	// 4. Set up a configuration map which contains the script for the job.
	// 5. Generate a job for deregistration with the configuration map mounted as a volume.
	if err := r.Client.Create(ctx, constructServiceAccount(), &client.CreateOptions{}); err != nil {
		log.Error(err, "unable to create service account")
		return err
	}
	if err := r.Client.Create(ctx, constructClusterRole(), &client.CreateOptions{}); err != nil {
		log.Error(err, "unable to create cluster role")
		return err
	}
	if err := r.Client.Create(ctx, constructClusterRoleBinding(), &client.CreateOptions{}); err != nil {
		log.Error(err, "unable to create cluster rolebinding")
		return err
	}
	if err := r.Client.Create(ctx, constructConfigMap(), &client.CreateOptions{}); err != nil {
		log.Error(err, "unable to create cluster configmap")
		return err
	}
	if err := r.Client.Create(ctx, constructJobForClusterDeregister(), &client.CreateOptions{}); err != nil {
		log.Error(err, "unable to create job for cluster deregister")
		return err
	}
	return nil
}

func (r *Reconciler) updateClusterDeregisterStatus(status string, ctx context.Context, cluster *hubv1alpha1.Cluster) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := r.Get(ctx, types.NamespacedName{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
		}, cluster)
		if err != nil {
			return err
		}
		// Todo: should be uncommented once the cluster CRD/Type has been updated on the controller side.
		// cluster.Status.RegistrationStatus = status
		return r.Status().Update(ctx, cluster)
	})
	if err != nil {
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

func constructClusterRole() *rbacv1.ClusterRole {
	// Todo: work on policies
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterRoleBindingName,
			Namespace: ControlPlaneNamespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{
					"",
					"batch",
					"extensions",
					"apps",
					"rbac.authorization.k8s.io",
					"admissionregistration.k8s.io",
					"scheduling.k8s.io",
				},
				Resources: []string{"*"},
				Verbs: []string{
					"get",
					"list",
					"patch",
					"update",
					"delete",
				},
			},
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
	// delete service account/cluster role/cluster rb after everything
	// remove cluster finalizer
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
	// Todo: change docker image
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
							{
								Name:  "PROJECT_NAMESPACE",
								Value: ProjectNamespace,
							},
							{
								Name:  "HUB_ENDPOINT",
								Value: HubEndpoint,
							},
							{
								Name:  "CLUSTER_NAME",
								Value: ClusterName,
							},
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
