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

func (r *Reconciler) CreateDeregisterJob(ctx context.Context) error {
	log := logger.FromContext(ctx).WithName("cluster-deregister")
	// 1. create service account
	sa := constructServiceAccount()
	if err := r.Client.Create(ctx, sa, &client.CreateOptions{}); err != nil {
		log.Error(err, "unable to create service account")
		return err
	}
	// 2. create cluster rolebinding
	rolebinding := ConstructClusterRoleBinding()
	if err := r.Client.Create(ctx, rolebinding, &client.CreateOptions{}); err != nil {
		log.Error(err, "unable to create cluster rolebinding")
		return err
	}
	// 3. create job
	job := ConstructJobForClusterDeregister()
	if err := r.Client.Create(ctx, job, &client.CreateOptions{}); err != nil {
		log.Error(err, "unable to create job for cluster deregister")
		return err
	}
	return nil
}

func constructServiceAccount() *corev1.ServiceAccount {
	svcAcc := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "helm-uninstall-sa",
			Namespace: ControlPlaneNamespace,
		},
	}
	return svcAcc
}

func ConstructClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	clusterRole := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "helm-uninstall-rolebinding",
			Namespace: ControlPlaneNamespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      "helm-uninstall-sa",
			Namespace: ControlPlaneNamespace,
		}},
	}
	return clusterRole
}

func ConstructJobForClusterDeregister() *batchv1.Job {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sequential-jobs",
			Namespace: ControlPlaneNamespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sequential-jobs",
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "cluster-admin-test",
					InitContainers: []corev1.Container{{
						Name:    "helm-uninstall-job",
						Image:   "dtzar/helm-kubectl",
						Command: []string{"/bin/sh"},
						Args: []string{
							"-c",
							"helm uninstall kubeslice-worker --namespace kubeslice-system --debug 2>&1 | tee /shared-volume/status.txt",
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								MountPath: "/tmp",
								Name:      "shared-volume",
							},
						}},
					},
					Containers: []corev1.Container{{
						Name:  "kubectl",
						Image: "dtzar/helm-kubectl",
						Command: []string{
							"/bin/bash",
							"sleep 20m",
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								MountPath: "/tmp",
								Name:      "shared-volume",
							},
						}},
					},
					Volumes: []corev1.Volume{{
						Name: "shared-volume",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						}},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}
	return job
}
