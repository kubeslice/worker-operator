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
	"os"
	"path"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
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
	err := r.updateClusterDeregisterStatus(ctx, cluster, hubv1alpha1.RegistrationStatusDeregisterInProgress)
	if err != nil {
		log.Error(err, "error updating status of deregistration on the controller")
		return err
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
	// Create a cluster role.
	if err := r.MeshClient.Create(ctx, constructClusterRole(operatorClusterRole), &client.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			log.Info("cluster role already exists", "clusterrole", clusterRoleName)
		} else {
			log.Error(err, "unable to create cluster role")
			return err
		}
	}
	// Bind the service account with the cluster role.
	if err := r.MeshClient.Create(ctx, constructClusterRoleBinding(), &client.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			log.Info("cluster rolebinding already exists", "clusterrolebinding", clusterRoleBindingName)
		} else {
			log.Error(err, "unable to create cluster rolebinding")
			return err
		}
	}
	// get configmap data
	data, err := getConfigmapData()
	if err != nil {
		log.Error(err, "unable to fetch configmap data")
		return err
	}
	// Set up a configuration map which contains the script for the job.
	if err := r.MeshClient.Create(ctx, constructConfigMap(data), &client.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			log.Info("cluster configmap already exists", "configmap", clusterDeregisterConfigMap)
		} else {
			log.Error(err, "unable to create cluster configmap")
			return err
		}
	}

	// get the job
	jobRef := types.NamespacedName{
		Name:      deregisterJobName,
		Namespace: ControlPlaneNamespace,
	}
	deregisterJob := constructJobForClusterDeregister()
	if err := r.MeshClient.Get(ctx, jobRef, deregisterJob); err == nil {
		// Job is already present we need to delete it
		err = r.MeshClient.Delete(ctx, deregisterJob)
		if err != nil {
			log.Error(err, "error while deleting job object", "job", jobRef.Name)
			return err
		}
	}

	// Generate a job for deregistration with the configuration map mounted as a volume.
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

func (r *Reconciler) updateClusterDeregisterStatus(ctx context.Context, cluster *hubv1alpha1.Cluster, status hubv1alpha1.RegistrationStatus) error {
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

func (r *Reconciler) updateRegistrationStatusAndDeregisterInProgress(ctx context.Context, cluster *hubv1alpha1.Cluster, status hubv1alpha1.RegistrationStatus, isDeregisterInProgress bool) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := r.Get(ctx, types.NamespacedName{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
		}, cluster)
		if err != nil {
			return err
		}
		cluster.Status.RegistrationStatus = status
		cluster.Status.IsDeregisterInProgress = isDeregisterInProgress
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

func constructClusterRole(clusterRole *rbacv1.ClusterRole) *rbacv1.ClusterRole {
	clusterRoleCopy := &rbacv1.ClusterRole{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName, Namespace: ControlPlaneNamespace},
		Rules:      clusterRole.Rules,
	}

	return clusterRoleCopy
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

func getConfigmapScriptPath(file string) string {
	dir := os.Getenv("SCRIPT_PATH")
	if dir != "" {
		return path.Join(dir, file+".sh")
	}

	return path.Join(scriptPath, file+".sh")
}

func getConfigmapData() (string, error) {
	fileName := "cleanup"
	data, err := os.ReadFile(getConfigmapScriptPath(fileName))
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
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: serviceAccountName,
					Containers: []corev1.Container{{
						Name:  serviceAccountName,
						Image: "aveshadev/worker-installer:latest",
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
