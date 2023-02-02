package controllers

import (
	"context"
	"reflect"
	"time"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	"github.com/kubeslice/worker-operator/pkg/cluster"
	"github.com/kubeslice/worker-operator/pkg/events"
	"github.com/kubeslice/worker-operator/pkg/logger"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	GCP   string = "gcp"
	AWS   string = "aws"
	AZURE string = "azure"

	ReconcileInterval = 10 * time.Second
)

type ClusterReconciler struct {
	client.Client
	MeshClient    client.Client
	EventRecorder *events.EventRecorder
}

func (r *ClusterReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logger.FromContext(ctx)

	cr, err := r.getCluster(ctx, req)
	if cr == nil {
		return reconcile.Result{}, err
	}

	log.Info("got cluster CR from hub", "cluster", cr)

	// Populate NodeIPs if not already updated
	// Only needed to do the initial update. Later updates will be done by node reconciler
	if cr.Spec.NodeIPs == nil || len(cr.Spec.NodeIPs) == 0 {
		nodeIPs, err := cluster.GetNodeIP(r.MeshClient)
		if err != nil {
			log.Error(err, "Error Getting nodeIP")
			return reconcile.Result{}, err
		}
		cr.Spec.NodeIPs = nodeIPs
		if err := r.Update(ctx, cr); err != nil {
			log.Error(err, "Error updating to cluster spec on hub cluster")
			return reconcile.Result{}, err
		}
		return reconcile.Result{RequeueAfter: ReconcileInterval}, nil
	}

	cl := cluster.NewCluster(r.MeshClient, clusterName)
	clusterInfo, err := cl.GetClusterInfo(ctx)
	if err != nil {
		log.Error(err, "Error getting clusterInfo")
		return reconcile.Result{}, err
	}
	if clusterInfo.ClusterProperty.GeoLocation.CloudProvider == GCP || clusterInfo.ClusterProperty.GeoLocation.CloudProvider == AWS || clusterInfo.ClusterProperty.GeoLocation.CloudProvider == AZURE {
		cr.Spec.ClusterProperty.GeoLocation.CloudRegion = clusterInfo.ClusterProperty.GeoLocation.CloudRegion
		cr.Spec.ClusterProperty.GeoLocation.CloudProvider = clusterInfo.ClusterProperty.GeoLocation.CloudProvider
		if err := r.Update(ctx, cr); err != nil {
			log.Error(err, "Error updating ClusterProperty on hub cluster")
			return reconcile.Result{}, err
		}
	}
	cniSubnet, err := cl.GetNsmExcludedPrefix(ctx, "nsm-config", "kubeslice-system")
	if err != nil {
		log.Error(err, "Error getting cni Subnet")
		return reconcile.Result{}, err
	}
	if !reflect.DeepEqual(cr.Status.CniSubnet, cniSubnet) {
		cr.Status.CniSubnet = cniSubnet
		if err := r.Status().Update(ctx, cr); err != nil {
			log.Error(err, "Error updating cniSubnet to cluster status on hub cluster")
			return reconcile.Result{}, err
		}
	}

	// TODO: update namespaces on hub
	// TODO: post dashboard creds

	return reconcile.Result{RequeueAfter: ReconcileInterval}, nil
}

func (r *ClusterReconciler) getCluster(ctx context.Context, req reconcile.Request) (*hubv1alpha1.Cluster, error) {
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

func (r *ClusterReconciler) InjectClient(c client.Client) error {
	r.Client = c
	return nil
}
