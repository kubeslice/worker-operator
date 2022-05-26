package networkpolicy

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/go-logr/logr"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	slicepkg "github.com/kubeslice/worker-operator/controllers/slice"
	"github.com/kubeslice/worker-operator/pkg/events"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SliceReconciler reconciles a Slice object
type NetpolReconciler struct {
	client.Client
	EventRecorder   *events.EventRecorder
	Scheme          *runtime.Scheme
	Log             logr.Logger
	privateIPBlocks []*net.IPNet
}

func (r *NetpolReconciler) initPrivateIPBlocks() error {
	for _, cidr := range []string{
		"127.0.0.0/8",    // IPv4 loopback
		"10.0.0.0/8",     // RFC1918
		"172.16.0.0/12",  // RFC1918
		"192.168.0.0/16", // RFC1918
		"169.254.0.0/16", // RFC3927 link-local
		"::1/128",        // IPv6 loopback
		"fe80::/10",      // IPv6 link-local
		"fc00::/7",       // IPv6 unique local addr
	} {
		_, block, err := net.ParseCIDR(cidr)
		if err != nil {
			r.Log.Error(err, "parse error on %q: %v", cidr, err)
			return err
		}
		r.privateIPBlocks = append(r.privateIPBlocks, block)
	}
	return nil
}

func (c *NetpolReconciler) getSliceNameFromNsOfNetPol(ns string) (string, error) {
	namespace := corev1.Namespace{}
	err := c.Client.Get(context.Background(), types.NamespacedName{Name: ns}, &namespace)
	if err != nil {
		c.Log.Error(err, "error while retrieving namespace")
		return "", err
	}
	return namespace.Labels[slicepkg.ApplicationNamespaceSelectorLabelKey], nil
}

func (r *NetpolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("netpol reconciler", req.NamespacedName)
	netpol := networkingv1.NetworkPolicy{}
	if err := r.Get(ctx, req.NamespacedName, &netpol); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Return and don't requeue
			log.Info("networkpolicy not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get networkpolicy")
		return ctrl.Result{}, err
	}
	if err := r.initPrivateIPBlocks(); err != nil {
		return ctrl.Result{}, err
	}

	//get the sliceName from namespace label
	sliceName, err := r.getSliceNameFromNsOfNetPol(req.Namespace)
	if err != nil {
		log.Error(err, "error while retrieving labels from namespace")
		return ctrl.Result{}, err
	}
	if len(sliceName) == 0 {
		// random netpol being added to namespace which is not part of slice
		log.Info(fmt.Sprintf("added network policy(%s) to namespace(%s) which is not part of slice.", netpol.Name, netpol.Namespace))
		return ctrl.Result{}, nil
	}

	//if this network policy is the one installed by slice reconciler, we ignore it
	if strings.EqualFold(sliceName+"-"+netpol.Namespace, netpol.Name) {
		//early exit
		log.Info("added/modified network policy,ignoring since it is reconciled by slice reconciler")
		return ctrl.Result{}, nil
	}
	//get slice
	slice := &kubeslicev1beta1.Slice{}
	err = r.Get(context.Background(), types.NamespacedName{Name: sliceName, Namespace: "kubeslice-system"}, slice)
	if err != nil {
		log.Error(err, fmt.Sprintf("error while retrieving slice(%s/%s)", "kubeslice-system", sliceName))
		return ctrl.Result{}, err
	}

	//extra network policy being added , compare and raise an event
	clusterName := os.Getenv("CLUSTER_NAME")
	r.EventRecorder.Record(
		&events.Event{
			Object:    slice,
			EventType: events.EventTypeWarning,
			Reason:    "Added",
			Message:   fmt.Sprintf("added network policy(%s) in slice(%s/%s) of cluster(%s)", netpol.Name, slice.Namespace, slice.Name, clusterName),
		},
	)
	log.Info(fmt.Sprintf("added network policy(%s) in slice(%s/%s) of cluster(%s)", netpol.Name, netpol.Namespace, slice.Name, clusterName))

	return r.Compare(&netpol, slice)
}

func (c *NetpolReconciler) Compare(np *networkingv1.NetworkPolicy, slice *kubeslicev1beta1.Slice) (ctrl.Result, error) {
	var ApplicationNamespaces, err1 = c.GetAppNamespacesBySliceNameAndLabel(context.Background(), slice.Name, slicepkg.ApplicationNamespaceSelectorLabelKey)
	if err1 != nil {
		c.Log.Error(err1, "error while retrieving application namespaces by sliceName")
		return ctrl.Result{}, err1
	}
	var AllowedNamespaces, err2 = c.GetAllowedNamespacesBySliceNameAndLabel(context.Background(), slice,
		slicepkg.AllowedNamespaceSelectorLabelKey)
	if err2 != nil {
		c.Log.Error(err2, "error while retrieving allowed namespaces by sliceName")
		return ctrl.Result{}, err2
	}
	ingressRules := np.Spec.Ingress
	for _, ingressRule := range ingressRules {
		networkPolicyPeers := ingressRule.From
		for _, networkPolicyPeer := range networkPolicyPeers {
			if networkPolicyPeer.NamespaceSelector != nil {
				namespaceList := &corev1.NamespaceList{}
				listOpts := []client.ListOption{
					client.MatchingLabels(networkPolicyPeer.NamespaceSelector.MatchLabels),
				}
				err := c.Client.List(context.Background(), namespaceList, listOpts...)
				if err != nil {
					c.Log.Error(err, "error while retrieving namespace")
					return ctrl.Result{}, err
				}
				if namespaceList != nil && len(namespaceList.Items) > 0 {
					for _, item := range namespaceList.Items {
						// namespaces found but not in allowed namespaces or application namespaces list
						if !Contains(&ApplicationNamespaces, item.Name) && !Contains(&AllowedNamespaces, item.Name) {
							clusterName := os.Getenv("CLUSTER_NAME")
							// Record net pol modified event
							c.EventRecorder.Record(
								&events.Event{
									Object:    slice,
									EventType: events.EventTypeWarning,
									Reason:    "Scope widened with reason - namespace violation",
									Message:   fmt.Sprintf("widened scope with network policy(%s) in slice(%s/%s) of cluster(%s)", np.Name, slice.Namespace, slice.Name, clusterName),
								},
							)
							c.Log.Info(fmt.Sprintf("widened scope with network policy(%s) in slice(%s/%s) of cluster(%s)",
								np.Name,
								slice.Namespace,
								slice.Name, clusterName))
						}
					}
				}
			}
			if networkPolicyPeer.IPBlock != nil {
				ipBlock := networkPolicyPeer.IPBlock
				if ipBlock != nil {
					_, netpolNet, _ := net.ParseCIDR(ipBlock.CIDR)
					if c.isPrivateIP(netpolNet.IP) {
						clusterName := os.Getenv("CLUSTER_NAME")
						// Record net pol modified event

						c.EventRecorder.Record(
							&events.Event{
								Object:    slice,
								EventType: events.EventTypeWarning,
								Reason:    "Scope widened with reason - IPBlock violation",
								Message:   fmt.Sprintf("widened scope with network policy(%s) in slice(%s/%s) of cluster(%s)", np.Name, slice.Namespace, slice.Name, clusterName),
							},
						)

						c.Log.Info(fmt.Sprintf("widened scope with network policy(%s) in slice(%s/%s) of cluster("+
							"%s) : Reason(IPBlock violation)",
							np.Name,
							slice.Namespace,
							slice.Name, clusterName))
					}
				}
			}
		}
	}
	return ctrl.Result{}, nil
}

// GetAllowedNamespacesBySliceName gets namespaces
func (c *NetpolReconciler) GetAppNamespacesBySliceNameAndLabel(ctx context.Context, sliceName string,
	selectorLabelKey string) ([]string,
	error) {

	labelSelector := map[string]string{selectorLabelKey: sliceName}

	namespaces := corev1.NamespaceList{}
	listOpts := []client.ListOption{
		client.MatchingLabels(labelSelector),
	}
	err := c.Client.List(ctx, &namespaces, listOpts...)
	if err != nil {
		return nil, err
	}
	var namespaceNames []string
	for _, ns := range namespaces.Items {
		namespaceNames = append(namespaceNames, ns.Name)
	}
	return namespaceNames, nil
}

func (c *NetpolReconciler) GetAllowedNamespacesBySliceNameAndLabel(ctx context.Context, slice *kubeslicev1beta1.Slice,
	selectorLabelKey string) ([]string,
	error) {
	return slice.Status.SliceConfig.NamespaceIsolationProfile.AllowedNamespaces, nil
}

// Checks if the passed string is present in the first argument slice.
func Contains(s *[]string, e string) bool {
	for _, a := range *s {
		if strings.EqualFold(a, e) {
			return true
		}
	}
	return false
}

func (c *NetpolReconciler) isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	for _, block := range c.privateIPBlocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *NetpolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1.NetworkPolicy{}).
		Complete(r)
}
