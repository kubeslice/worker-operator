package cluster

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"time"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	"github.com/kubeslice/worker-operator/controllers"
	"github.com/kubeslice/worker-operator/pkg/cluster"
	"github.com/kubeslice/worker-operator/pkg/events"
	"github.com/kubeslice/worker-operator/pkg/logger"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	GCP   string = "gcp"
	AWS   string = "aws"
	AZURE string = "azure"

	ReconcileInterval = 2 * time.Minute
)

type Reconciler struct {
	client.Client
	MeshClient    client.Client
	EventRecorder *events.EventRecorder
}

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logger.FromContext(ctx).WithName("cluster-reconciler")
	ctx = logger.WithLogger(ctx, log)

	cr, err := r.getCluster(ctx, req)
	if cr == nil {
		return reconcile.Result{}, err
	}

	log.Info("got cluster CR from hub", "cluster", cr)

	// Post NodeIP and GeoLocation info only on first run or if the reconciler wasn't run for a while
	if cr.Status.ClusterHealth == nil || time.Since(cr.Status.ClusterHealth.LastUpdated.Time) > time.Minute {
		log.Info("updating cluster info on controller")
		if err := r.updateClusterInfo(ctx, cr); err != nil {
			log.Error(err, "unable to update cluster info")
		}
	}

	// Update dashboard creds if it hasn't already
	if !r.isDashboardCredsUpdated(ctx, cr) {
		if err := r.updateDashboardCreds(ctx, cr); err != nil {
			log.Error(err, "unable to update dashboard creds")
		}
	}
	firstTime := false
	if cr.Status.ClusterHealth == nil {
		firstTime = true
		cr.Status.ClusterHealth = &hubv1alpha1.ClusterHealth{}
	}

	if err := r.UpdateClusterHealthStatus(ctx, cr); err != nil {
		log.Error(err, "unable to update cluster health status")
	}
	if firstTime || time.Since(cr.Status.ClusterHealth.LastUpdated.Time) >= ReconcileInterval {
		cr.Status.ClusterHealth.LastUpdated = metav1.Now()
		if err := r.Status().Update(ctx, cr); err != nil {
			log.Error(err, "unable to update cluster CR")
		}
	}
	return reconcile.Result{RequeueAfter: ReconcileInterval}, nil
}

func (r *Reconciler) UpdateClusterHealthStatus(ctx context.Context, cr *hubv1alpha1.Cluster) error {
	log := logger.FromContext(ctx)
	cr.Status.ClusterHealth.ComponentStatuses = []hubv1alpha1.ComponentStatus{}
	cr.Status.ClusterHealth.ClusterHealthStatus = hubv1alpha1.ClusterHealthStatusNormal

	for _, c := range components {
		cs, err := r.getComponentStatus(ctx, &c)
		if err != nil {
			log.Error(err, "unable to fetch component status")
		}
		if cs != nil {
			cr.Status.ClusterHealth.ComponentStatuses = append(cr.Status.ClusterHealth.ComponentStatuses, *cs)
			if cs.ComponentHealthStatus != hubv1alpha1.ComponentHealthStatusNormal {
				cr.Status.ClusterHealth.ClusterHealthStatus = hubv1alpha1.ClusterHealthStatusWarning
			}
		}
	}

	return nil
}

func (r *Reconciler) getComponentStatus(ctx context.Context, c *component) (*hubv1alpha1.ComponentStatus, error) {
	log := logger.FromContext(ctx)
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.MatchingLabels(c.labels),
		client.InNamespace(c.ns),
	}
	if err := r.MeshClient.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods", "pod", c.name)
		return nil, err
	}

	pods := podList.Items
	cs := &hubv1alpha1.ComponentStatus{
		Component: c.name,
	}
	if len(pods) == 0 && c.ignoreMissing {
		return nil, nil
	}
	if len(pods) == 0 {
		log.Error(fmt.Errorf("No pods running"), "unhealthy", "pod", c.name)
		cs.ComponentHealthStatus = hubv1alpha1.ComponentHealthStatusError
		return cs, nil
	}

	for _, pod := range pods {
		if pod.Status.Phase != corev1.PodRunning {
			log.Info("pod is not healthy", "component", c.name)
			cs.ComponentHealthStatus = hubv1alpha1.ComponentHealthStatusError
			return cs, nil
		}
	}

	cs.ComponentHealthStatus = hubv1alpha1.ComponentHealthStatusNormal
	return cs, nil
}

func (r *Reconciler) updateClusterInfo(ctx context.Context, cr *hubv1alpha1.Cluster) error {
	log := logger.FromContext(ctx)
	cl := cluster.NewCluster(r.MeshClient, clusterName)
	clusterInfo, err := cl.GetClusterInfo(ctx)
	if err != nil {
		log.Error(err, "Error getting clusterInfo")
		return err
	}
	log.Info("got clusterinfo", "ci", clusterInfo)
	if clusterInfo.ClusterProperty.GeoLocation.CloudProvider == GCP || clusterInfo.ClusterProperty.GeoLocation.CloudProvider == AWS || clusterInfo.ClusterProperty.GeoLocation.CloudProvider == AZURE {
		cr.Spec.ClusterProperty.GeoLocation.CloudRegion = clusterInfo.ClusterProperty.GeoLocation.CloudRegion
		cr.Spec.ClusterProperty.GeoLocation.CloudProvider = clusterInfo.ClusterProperty.GeoLocation.CloudProvider
		if err := r.Update(ctx, cr); err != nil {
			log.Error(err, "Error updating ClusterProperty on hub cluster")
			return err
		}
	}
	cniSubnet, err := cl.GetNsmExcludedPrefix(ctx, "nsm-config", "kubeslice-system")
	if err != nil {
		log.Error(err, "Error getting cni Subnet")
		return err
	}
	if !reflect.DeepEqual(cr.Status.CniSubnet, cniSubnet) {
		cr.Status.CniSubnet = cniSubnet
		if err := r.Status().Update(ctx, cr); err != nil {
			log.Error(err, "Error updating cniSubnet to cluster status on hub cluster")
			return err
		}
	}

	// Populate NodeIPs if not already updated
	// Only needed to do the initial update. Later updates will be done by node reconciler
	if cr.Spec.NodeIPs == nil || len(cr.Spec.NodeIPs) == 0 {
		nodeIPs, err := cluster.GetNodeIP(r.MeshClient)
		if err != nil {
			log.Error(err, "Error Getting nodeIP")
			return err
		}
		cr.Spec.NodeIPs = nodeIPs
		if err := r.Update(ctx, cr); err != nil {
			log.Error(err, "Error updating to cluster spec on hub cluster")
			return err
		}
	}

	return nil
}

func (r *Reconciler) isDashboardCredsUpdated(ctx context.Context, cr *hubv1alpha1.Cluster) bool {
	return cr.Spec.ClusterProperty.Monitoring.KubernetesDashboard.Enabled
}

func (r *Reconciler) updateDashboardCreds(ctx context.Context, cr *hubv1alpha1.Cluster) error {
	log := logger.FromContext(ctx)
	log.Info("Updating kubernetes dashboard creds")

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      KubeSliceDashboardSA,
			Namespace: controllers.ControlPlaneNamespace,
		},
	}
	if err := r.MeshClient.Get(ctx, types.NamespacedName{Name: sa.Name, Namespace: controllers.ControlPlaneNamespace}, sa); err != nil {
		log.Error(err, "Error getting service account")
		return err
	}

	if len(sa.Secrets) == 0 {
		err := fmt.Errorf("ServiceAccount has no secret")
		log.Error(err, "Error getting service account secret")
		return err
	}

	secret := &corev1.Secret{}
	err := r.MeshClient.Get(ctx, types.NamespacedName{Name: sa.Secrets[0].Name, Namespace: controllers.ControlPlaneNamespace}, secret)
	if err != nil {
		log.Error(err, "Error getting service account's secret")
		return err
	}

	secretName := os.Getenv("CLUSTER_NAME") + HubSecretSuffix
	if secret.Data == nil {
		return fmt.Errorf("dashboard secret data is nil")
	}
	token, ok := secret.Data["token"]
	if !ok {
		return fmt.Errorf("token not present in dashboard secret")
	}
	cacrt, ok := secret.Data["ca.crt"]
	if !ok {
		return fmt.Errorf("ca.crt not present in dashboard secret")
	}

	secretData := map[string][]byte{
		"token":  token,
		"ca.crt": cacrt,
	}
	hubSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: os.Getenv("HUB_PROJECT_NAMESPACE"),
		},
		Data: secretData,
	}
	log.Info("creating secret on hub", "hubSecret", hubSecret.Name)
	err = r.Create(ctx, &hubSecret)
	if apierrors.IsAlreadyExists(err) {
		err = r.Update(ctx, &hubSecret)
	}

	if err != nil {
		return err
	}
	cr.Spec.ClusterProperty.Monitoring.KubernetesDashboard.Endpoint = os.Getenv("CLUSTER_ENDPOINT")
	cr.Spec.ClusterProperty.Monitoring.KubernetesDashboard.AccessToken = secretName
	cr.Spec.ClusterProperty.Monitoring.KubernetesDashboard.Enabled = true
	log.Info("Posting cluster creds to hub cluster", "cluster", os.Getenv("CLUSTER_NAME"))
	return r.Update(ctx, cr)
}

func (r *Reconciler) getCluster(ctx context.Context, req reconcile.Request) (*hubv1alpha1.Cluster, error) {
	hubCluster := &hubv1alpha1.Cluster{}
	log := logger.FromContext(ctx)
	err := r.Get(ctx, req.NamespacedName, hubCluster)
	// Request object not found, could have been deleted after reconcile request.
	if err != nil {
		if errors.IsNotFound(err) {
			// Return and don't requeue
			log.Info("cluster resource not found in hub. Ignoring since object must be deleted")
			return nil, nil
		}
		return nil, err
	}
	return hubCluster, nil
}

func (r *Reconciler) InjectClient(c client.Client) error {
	r.Client = c
	return nil
}
