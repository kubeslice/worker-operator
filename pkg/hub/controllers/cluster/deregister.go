package cluster

import (
	"context"
	"os"

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
)

// createDeregisterJob creates a job to uninstall worker-operator and notify controller
// about registration status.
func (r *Reconciler) createDeregisterJob(ctx context.Context, cluster *hubv1alpha1.Cluster) error {
	log := logger.FromContext(ctx).WithName("cluster-deregister")
	// Notify controller that the deregistration process of the cluster is in progress.
	err := r.updateClusterDeregisterStatus(hubv1alpha1.RegistrationStatusDeregisterInProgress, ctx, cluster)
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
	// Create a cluster role.
	if err := r.MeshClient.Create(ctx, constructClusterRole(), &client.CreateOptions{}); err != nil {
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

func (r *Reconciler) updateClusterDeregisterStatus(status hubv1alpha1.RegistrationStatus, ctx context.Context, cluster *hubv1alpha1.Cluster) error {
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

func (r *Reconciler) setIsDeregisterInProgress(ctx context.Context, cluster *hubv1alpha1.Cluster, val bool) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := r.Get(ctx, types.NamespacedName{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
		}, cluster)
		if err != nil {
			return err
		}
		cluster.Status.IsDeregisterInProgress = val
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
	clusterRole := &rbacv1.ClusterRole{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName, Namespace: ControlPlaneNamespace},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments", "daemonsets", "statefulsets", "replicasets"},
				Verbs:     []string{"get", "list", "patch", "update", "create", "delete"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"endpoints", "pods", "secrets", "services", "configmaps", "serviceaccounts", "namespaces"},
				Verbs:     []string{"delete", "list", "create", "get", "patch", "update"},
			},
			{
				APIGroups: []string{"rbac.authorization.k8s.io"},
				Resources: []string{"roles", "rolebindings", "clusterroles", "clusterrolebindings"},
				Verbs:     []string{"get", "list", "patch", "update", "create", "delete"},
			},
			{
				APIGroups: []string{"batch", "admissionregistration.k8s.io", "apiextensions.k8s.io", "scheduling.k8s.io"},
				Resources: []string{"*"},
				Verbs:     []string{"get", "list", "delete", "create", "watch"},
			},
			{
				APIGroups: []string{"spiffeid.spiffe.io"},
				Resources: []string{"spiffeids"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"spiffeid.spiffe.io"},
				Resources: []string{"spiffeids/status"},
				Verbs:     []string{"get", "patch", "update"},
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

func getConfigmapData() (string, error) {
	fileName := "scripts/cleanup.sh"
	data, err := os.ReadFile(fileName)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// delete service account/cluster role/cluster rb after everything
// remove cluster finalizer
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
