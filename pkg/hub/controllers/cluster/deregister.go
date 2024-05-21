/*
 *  Copyright (c) 2022 Avesha, Inc. All rights reserved.
 *
 *  SPDX-License-Identifier: Apache-2.0
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package cluster

import (
	"context"
	"errors"
	"os"
	"path"
	"time"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/pkg/logger"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	serviceAccountName         = "cluster-deregister-sa"
	cleanupContainer           = "cluster-cleanup"
	clusterRoleName            = "cluster-deregister-role"
	clusterRoleBindingName     = "cluster-deregister-rb"
	clusterDeregisterConfigMap = "cluster-deregister-cm"
	deregisterJobName          = "cluster-deregisteration-job"
	operatorClusterRoleName    = "kubeslice-manager-role"
)

const (
	scriptPath = "../../../../scripts"
)

// createDeregisterJob creates a job to uninstall worker-operator and notify controller
// about registration status.
func (r *Reconciler) createDeregisterJob(ctx context.Context, cluster *hubv1alpha1.Cluster) error {
	log := logger.FromContext(ctx).WithName("cluster-deregister")
	// Notify controller that the deregistration process of the cluster is in progress.
	err := r.updateRegistrationStatus(ctx, cluster, hubv1alpha1.RegistrationStatusDeregisterInProgress)
	if err != nil {
		log.Error(err, "error updating status of deregistration on the controller")
		return err
	}
	// If there are slice and slice gateways present at the worker cluster, throw an error
	maxRetries := 3
	slices := r.retryListSlice(ctx, maxRetries)
	if !slices {
		return errors.New("slice present in cluster, should delete it before deregistering cluster")
	}
	notPresentSliceGWs := r.retryListSliceGW(ctx, maxRetries)
	if !notPresentSliceGWs {
		return errors.New("slice gateway present in cluster, should delete it before deregistering cluster")
	}
	// Create a service account.
	if err := r.MeshClient.Create(ctx, constructServiceAccount(), &client.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			log.Info("service account already exists", "sa", serviceAccountName)
		} else {
			log.Error(err, "unable to create service account")
			return err
		}
	}
	// get operator clusterrole
	operatorClusterRole, err := r.getOperatorClusterRole(ctx)
	if err != nil {
		log.Error(err, "unable to fetch operator clusterrole")
		return err
	}
	ns := &corev1.Namespace{}
	nsRef := types.NamespacedName{
		Name: ControlPlaneNamespace,
	}
	err = r.MeshClient.Get(ctx, nsRef, ns)
	if err != nil {
		log.Error(err, "Failed to get worker operator namespace info")
		return err
	}
	// We set the namespace as the owner reference for the cluster role and cluster role binding.
	// This ensures that when the namespace is deleted, the corresponding cluster role and cluster role binding are also deleted.
	ownerUid := ns.UID
	// Create a cluster role.
	if err := r.MeshClient.Create(ctx, constructClusterRole(operatorClusterRole, ownerUid), &client.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			log.Info("cluster role already exists", "clusterrole", clusterRoleName)
		} else {
			log.Error(err, "unable to create cluster role")
			return err
		}
	}
	// Bind the service account with the cluster role.
	if err := r.MeshClient.Create(ctx, constructClusterRoleBinding(ownerUid), &client.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			log.Info("cluster rolebinding already exists", "clusterrolebinding", clusterRoleBindingName)
		} else {
			log.Error(err, "unable to create cluster rolebinding")
			return err
		}
	}
	// get the cleanup script as a long string and construct a configmap from it
	data, err := getCleanupScript()
	if err != nil {
		log.Error(err, "unable to fetch configmap data")
		return err
	}

	cleanupConfigMap := constructConfigMap(data)
	cmRef := types.NamespacedName{
		Name:      clusterDeregisterConfigMap,
		Namespace: ControlPlaneNamespace,
	}
	// check if the cleanup configmap already exists, delete it to ensure it has correct data
	if err := r.MeshClient.Get(ctx, cmRef, cleanupConfigMap); err == nil {
		err = r.MeshClient.Delete(ctx, cleanupConfigMap)
		if err != nil {
			log.Error(err, "error while deleting job object", "job", cmRef.Name)
			return err
		}
	}
	// constructing cleanup configmap
	if err := r.MeshClient.Create(ctx, cleanupConfigMap, &client.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			log.Info("cluster configmap already exists", "configmap", clusterDeregisterConfigMap)
		} else {
			log.Error(err, "unable to create cluster configmap")
			return err
		}
	}

	// check if the cleanup job already exists, delete it to ensure it has correct spec
	jobRef := types.NamespacedName{
		Name:      deregisterJobName,
		Namespace: ControlPlaneNamespace,
	}
	deregisterJob := constructJobForClusterDeregister()
	if err := r.MeshClient.Get(ctx, jobRef, deregisterJob); err == nil {
		err = r.MeshClient.Delete(ctx, deregisterJob)
		if err != nil {
			log.Error(err, "error while deleting job object", "job", jobRef.Name)
			return err
		}
	}

	// generate a job for deregistration with the cleanup configmap mounted as a volume
	if err := r.MeshClient.Create(ctx, deregisterJob, &client.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			log.Info("cluster deregister job already exists", "job", clusterDeregisterConfigMap)
		} else {
			log.Error(err, "unable to create job for cluster deregister")
			return err
		}
	}
	return nil
}

func (r *Reconciler) updateRegistrationStatus(ctx context.Context, cluster *hubv1alpha1.Cluster, status hubv1alpha1.RegistrationStatus) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := r.Get(ctx, types.NamespacedName{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
		}, cluster)
		if err != nil {
			return err
		}
		cluster.Status.RegistrationStatus = status
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

func (r *Reconciler) getOperatorClusterRole(ctx context.Context) (*rbacv1.ClusterRole, error) {
	operatorClusterRole := &rbacv1.ClusterRole{}
	err := r.MeshClient.Get(ctx, types.NamespacedName{Name: operatorClusterRoleName}, operatorClusterRole)
	if err != nil {
		return nil, err
	}
	return operatorClusterRole, nil
}

func constructClusterRole(clusterRole *rbacv1.ClusterRole, ownerUid types.UID) *rbacv1.ClusterRole {
	clusterRoleCopy := &rbacv1.ClusterRole{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName, Namespace: ControlPlaneNamespace},
		Rules:      clusterRole.Rules,
	}
	ownerRef := metav1.OwnerReference{
		APIVersion: "v1",
		Kind:       "Namespace",
		Name:       ControlPlaneNamespace,
		UID:        types.UID(ownerUid),
	}
	clusterRoleCopy.ObjectMeta.OwnerReferences = append(clusterRoleCopy.ObjectMeta.OwnerReferences, ownerRef)
	return clusterRoleCopy
}

func constructClusterRoleBinding(ownerUid types.UID) *rbacv1.ClusterRoleBinding {
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
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
	ownerRef := metav1.OwnerReference{
		APIVersion: "v1",
		Kind:       "Namespace",
		Name:       ControlPlaneNamespace,
		UID:        types.UID(ownerUid),
	}
	clusterRoleBinding.ObjectMeta.OwnerReferences = append(clusterRoleBinding.ObjectMeta.OwnerReferences, ownerRef)
	return clusterRoleBinding
}

func getCleanupScript() (string, error) {
	fileName := "cleanup.sh"
	filePath := path.Join(scriptPath, fileName)
	dir := os.Getenv("SCRIPT_PATH")
	if dir != "" {
		filePath = path.Join(dir, fileName)
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func constructConfigMap(data string) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterDeregisterConfigMap,
			Namespace: ControlPlaneNamespace,
		},
		Data: map[string]string{
			"kubeslice-cleanup.sh": data,
		}}
	return cm
}

func constructJobForClusterDeregister() *batchv1.Job {
	workerInstallerImage := os.Getenv("WORKER_INSTALLER_IMAGE")
	if workerInstallerImage == "" {
		workerInstallerImage = WORKER_INSTALLER_DEFAULT_IMAGE
	}
	backOffLimit := int32(0)
	ttlSecondsAfterFinished := int32(600)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deregisterJobName,
			Namespace: ControlPlaneNamespace,
			Labels: map[string]string{
				"kubeslice-manager/api-gateway": "cluster-deregister-job",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backOffLimit,
			TTLSecondsAfterFinished: &ttlSecondsAfterFinished,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: deregisterJobName,
					Labels: map[string]string{
						"kubeslice-manager/api-gateway": "cluster-deregister-job",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: serviceAccountName,
					Containers: []corev1.Container{{
						Name:  cleanupContainer,
						Image: workerInstallerImage,
						Command: []string{
							"/bin/bash",
							"/tmp/kubeslice-cleanup.sh",
						},
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
								LocalObjectReference: corev1.LocalObjectReference{
									Name: clusterDeregisterConfigMap,
								},
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

func (r *Reconciler) retryListSlice(ctx context.Context, maxRetries int) bool {
	retries := 0
	for {
		listOpts := []client.ListOption{}
		slice := kubeslicev1beta1.SliceList{}
		err := r.MeshClient.List(ctx, &slice, listOpts...)
		if err != nil {
			// slice listing error
			retries++
			if retries > maxRetries {
				return false
			}
			time.Sleep(5 * time.Second)
			continue
		}

		if len(slice.Items) != 0 {
			retries++
			if retries > maxRetries {
				return false
			}
			time.Sleep(5 * time.Second)
			continue
		}
		// slice not present
		return true
	}
}
func (r *Reconciler) retryListSliceGW(ctx context.Context, maxRetries int) bool {
	retries := 0
	for {
		listOpts := []client.ListOption{}
		sliceGw := kubeslicev1beta1.SliceGatewayList{}
		err := r.MeshClient.List(ctx, &sliceGw, listOpts...)
		if err != nil {
			// slice gateway listing error
			retries++
			if retries > maxRetries {
				return false
			}
			time.Sleep(5 * time.Second)
			continue
		}

		if len(sliceGw.Items) != 0 {
			retries++
			if retries > maxRetries {
				return false
			}
			time.Sleep(5 * time.Second)
			continue
		}
		// slicegw not present
		return true
	}
}
