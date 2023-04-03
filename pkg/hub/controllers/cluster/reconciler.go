package cluster

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"time"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"github.com/kubeslice/worker-operator/controllers"
	"github.com/kubeslice/worker-operator/pkg/cluster"
	"github.com/kubeslice/worker-operator/pkg/logger"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	GCP   string = "gcp"
	AWS   string = "aws"
	AZURE string = "azure"

	ReconcileInterval = 120 * time.Second
)

type Reconciler struct {
	client.Client
	MeshClient    client.Client
	EventRecorder events.EventRecorder
}

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logger.FromContext(ctx).WithName("cluster-reconciler")
	ctx = logger.WithLogger(ctx, log)
	cr, err := r.getCluster(ctx, req)
	if cr == nil {
		return reconcile.Result{}, err
	}
	log.Info("got cluster CR from hub", "cluster", cr)
	cl := cluster.NewCluster(r.MeshClient, cr.Name)
	res, err, requeue := r.updateClusterCloudProviderInfo(ctx, cr, cl)
	if err != nil {
		log.Error(err, "unable to update cloud provider info to cluster")
	}
	if requeue {
		return res, err
	}
	res, err, requeue = r.updateCNISubnetConfig(ctx, cr, cl)
	if err != nil {
		log.Error(err, "unable to update cni subnet config to cluster")
	}
	if requeue {
		return res, err
	}
	res, err, requeue = r.updateNodeIps(ctx, cr)
	if err != nil {
		log.Error(err, "unable to update node ips to cluster")
	}
	if requeue {
		return res, err
	}
	// Update dashboard creds if it hasn't already (only one time event)
	if !r.isDashboardCredsUpdated(ctx, cr) {
		if err := r.updateDashboardCreds(ctx, cr); err != nil {
			log.Error(err, "unable to update dashboard creds")
			return reconcile.Result{}, err
		} else {
			log.Info("Dashboard creds updated in hub")
			return reconcile.Result{Requeue: true}, nil
		}
	}
	if cr.Status.ClusterHealth == nil {
		cr.Status.ClusterHealth = &hubv1alpha1.ClusterHealth{}
	}

	if err := r.updateClusterHealthStatus(ctx, cr); err != nil {
		log.Error(err, "unable to update cluster health status")
	}

	cr.Status.ClusterHealth.LastUpdated = metav1.Now()
	if err := r.Status().Update(ctx, cr); err != nil {
		log.Error(err, "unable to update cluster CR")
		return reconcile.Result{}, err
	}

	return reconcile.Result{RequeueAfter: ReconcileInterval}, nil
}

func (r *Reconciler) updateClusterHealthStatus(ctx context.Context, cr *hubv1alpha1.Cluster) error {
	log := logger.FromContext(ctx)
	csList := []hubv1alpha1.ComponentStatus{}
	var chs hubv1alpha1.ClusterHealthStatus = hubv1alpha1.ClusterHealthStatusNormal

	for _, c := range components {
		cs, err := r.getComponentStatus(ctx, &c)
		if err != nil {
			log.Error(err, "unable to fetch component status")
		}
		if cs != nil {
			csList = append(csList, *cs)
			if cs.ComponentHealthStatus != hubv1alpha1.ComponentHealthStatusNormal {
				chs = hubv1alpha1.ClusterHealthStatusWarning
				log.Info("Component unhealthy", "component", c.name)
			}
		}
	}

	// Cluster health changed, raise event
	if cr.Status.ClusterHealth.ClusterHealthStatus != chs {
		if chs == hubv1alpha1.ClusterHealthStatusNormal {
			log.Info("cluster health is back to normal")
			err := r.EventRecorder.RecordEvent(ctx, &events.Event{
				Object:            cr,
				Name:              events.EventClusterHealthy,
				ReportingInstance: "cluster_reconciler",
			})
			if err != nil {
				log.Error(err, "unable to record event for health check")
			}
		} else if chs == hubv1alpha1.ClusterHealthStatusWarning {
			log.Info("cluster health is in warning state")
			err := r.EventRecorder.RecordEvent(ctx, &events.Event{
				Object:            cr,
				Name:              events.EventClusterUnhealthy,
				ReportingInstance: "cluster_reconciler",
			})
			if err != nil {
				log.Error(err, "unable to record event for health check")
			}
		}
	}

	cr.Status.ClusterHealth.ComponentStatuses = csList
	cr.Status.ClusterHealth.ClusterHealthStatus = chs

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

func (r *Reconciler) updateClusterCloudProviderInfo(ctx context.Context, cr *hubv1alpha1.Cluster, cl cluster.ClusterInterface) (ctrl.Result, error, bool) {
	log := logger.FromContext(ctx)
	clusterInfo, err := cl.GetClusterInfo(ctx)
	if err != nil {
		log.Error(err, "Error getting clusterInfo")
		return ctrl.Result{}, err, true
	}
	log.Info("got clusterinfo", "ci", clusterInfo)
	toUpdate := false
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := r.Get(ctx, types.NamespacedName{
			Name:      cr.Name,
			Namespace: cr.Namespace,
		}, cr)
		if err != nil {
			return err
		}
		cloudProvider := clusterInfo.ClusterProperty.GeoLocation.CloudProvider
		cloudRegion := clusterInfo.ClusterProperty.GeoLocation.CloudRegion
		if cloudProvider == GCP || cloudProvider == AWS || cloudProvider == AZURE {
			// compare the current cloud region and provider with values stored in cluster spec, if not same then update
			if cloudRegion != cr.Spec.ClusterProperty.GeoLocation.CloudRegion || cloudProvider != cr.Spec.ClusterProperty.GeoLocation.CloudProvider {
				log.Info("updating Cluster's cloud info", "cloudProvider", cloudProvider, "cloudRegion", cloudRegion)
				cr.Spec.ClusterProperty.GeoLocation.CloudProvider = cloudProvider
				cr.Spec.ClusterProperty.GeoLocation.CloudRegion = cloudRegion
				toUpdate = true
				return r.Update(ctx, cr)
			}
		}
		return nil
	})
	if err != nil {
		log.Error(err, "Failed to update Cloud Provider Info on hub cluster")
		return ctrl.Result{}, err, true
	}
	if toUpdate {
		return ctrl.Result{Requeue: true}, nil, true
	}
	return ctrl.Result{}, nil, false
}

func (r *Reconciler) updateCNISubnetConfig(ctx context.Context, cr *hubv1alpha1.Cluster, cl cluster.ClusterInterface) (ctrl.Result, error, bool) {
	log := logger.FromContext(ctx)
	toUpdate := false
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := r.Get(ctx, types.NamespacedName{
			Name:      cr.Name,
			Namespace: cr.Namespace,
		}, cr)
		if err != nil {
			return err
		}
		cniSubnet, err := cl.GetNsmExcludedPrefix(ctx, "nsm-config", "kubeslice-system")
		if err != nil {
			log.Error(err, "Failed to get nsm config")
			return err
		}
		if !reflect.DeepEqual(cr.Status.CniSubnet, cniSubnet) {
			cr.Status.CniSubnet = cniSubnet
			toUpdate = true
			return r.Status().Update(ctx, cr)
		}
		return nil
	})
	if err != nil {
		log.Error(err, "Error updating cniSubnet to cluster status on hub cluster")
		return ctrl.Result{}, err, true
	}
	if toUpdate {
		return ctrl.Result{Requeue: true}, nil, true
	}
	return ctrl.Result{}, nil, false
}

func (r *Reconciler) updateNodeIps(ctx context.Context, cr *hubv1alpha1.Cluster) (ctrl.Result, error, bool) {
	log := logger.FromContext(ctx)
	toUpdate := false
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := r.Get(ctx, types.NamespacedName{
			Name:      cr.Name,
			Namespace: cr.Namespace,
		}, cr)
		if err != nil {
			return err
		}
		nodeIPs, err := cluster.GetNodeIP(r.MeshClient)
		if err != nil {
			log.Error(err, "Error Getting nodeIP")
			return err
		}
		// Populate NodeIPs if not already updated
		// OR gets the current nodeIP in use from controller cluster CR and compares it with
		// the current nodeIpList (nodeIpList contains list of externalIPs or internalIPs)
		// if the nodeIP is no longer available, we update the cluster CR on controller cluster
		if cr.Status.NodeIPs == nil || len(cr.Status.NodeIPs) == 0 ||
			!validatenodeips(nodeIPs, cr.Status.NodeIPs) || !isValidNodeIpList(nodeIPs) {
			log.Info("Mismatch in node IP", "IP in use", cr.Status.NodeIPs, "IP to be used", nodeIPs)
			cr.Status.NodeIPs = nodeIPs
			toUpdate = true
			return r.Status().Update(ctx, cr)
		}
		return nil
	})
	if err != nil {
		log.Error(err, "Error updating to node ip's on hub cluster")
		return ctrl.Result{}, err, true
	}
	// TODO raise event to cluster CR that node ip is updated
	if toUpdate {
		return ctrl.Result{Requeue: true}, nil, true
	}
	return ctrl.Result{}, nil, false
}

// total -> external ip list of nodes in the k8s cluster
// current -> ip list present in nodeIPs of cluster cr
func validatenodeips(total, current []string) bool {
	if len(total) != len(current) {
		return false
	}
	for i := range total {
		if total[i] != current[i] {
			return false
		}
	}
	return true
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

func isValidNodeIpList(nodeIPs []string) bool {
	for _, nodeIP := range nodeIPs {
		if nodeIP == "" {
			return false
		}
	}
	return true
}

func (r *Reconciler) InjectClient(c client.Client) error {
	r.Client = c
	return nil
}
