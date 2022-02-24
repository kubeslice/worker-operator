package controllers

import (
	"context"
	goerrors "errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	nsmv1alpha1 "github.com/networkservicemesh/networkservicemesh/k8s/pkg/apis/networkservice/v1alpha1"

	"bitbucket.org/realtimeai/kubeslice-operator/internal/logger"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	nsmDataplaneVpp                 string = "vpp"
	nsmDataplaneKernel              string = "kernel"
	nsmVppDataplaneCfgStr           string = "nsm-vpp-plane"
	nsmKernelDataplaneCfgStr        string = "nsm-kernel-plane"
	sliceRouterDeploymentNamePrefix string = "vl3-slice-router-"
)

func labelsForSliceRouterDeployment(name string) map[string]string {
	return map[string]string{
		"networkservicemesh.io/app":  "vl3-nse-" + name,
		"networkservicemesh.io/impl": "vl3-service-" + name,
		"avesha.io/pod-type":         "router",
		"avesha.io/slice":            name,
	}
}

func getSliceRouterSidecarImageAndPullPolicy() (string, corev1.PullPolicy) {
	image := "nexus.dev.aveshalabs.io/mesh-netops:1.0.0"
	pullPolicy := corev1.PullAlways

	if len(sliceRouterSidecarImage) > 0 {
		image = sliceRouterSidecarImage
	}
	if len(sliceRouterSidecarImagePullPolicy) > 0 {
		pullPolicy = corev1.PullPolicy(sliceRouterSidecarImagePullPolicy)
	}

	return image, pullPolicy
}

func (r *SliceReconciler) getNsmDataplaneMode(ctx context.Context, slice *meshv1beta1.Slice) (string, error) {
	log := logger.FromContext(ctx).WithValues("type", "SliceRouter")

	vppPodList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(slice.Namespace),
		client.MatchingLabels{"app": nsmVppDataplaneCfgStr},
	}
	if err := r.List(ctx, vppPodList, listOpts...); err != nil {
		log.Error(err, "Failed to list nsm vpp dataplane pods")
		return "", err
	}
	if len(vppPodList.Items) > 0 {
		return nsmDataplaneVpp, nil
	}

	return nsmDataplaneKernel, nil
}

func getClusterPrefixPool(sliceSubnet string, ipamOctet string) string {
	octetList := strings.Split(sliceSubnet, ".")
	octetList[2] = ipamOctet
	octetList[3] = "0/24"
	return strings.Join(octetList, ".")
}

func (r *SliceReconciler) getContainerSpecForSliceRouter(s *meshv1beta1.Slice, ipamOctet string, dataplane string) corev1.Container {
	vl3Image := "nexus.dev.aveshalabs.io/avesha/vl3_ucnf-nse:1.0.0"
	vl3ImagePullPolicy := corev1.PullAlways

	if len(vl3RouterImage) != 0 {
		vl3Image = vl3RouterImage
	}

	if len(vl3RouterPullPolicy) != 0 {
		vl3ImagePullPolicy = corev1.PullPolicy(vl3RouterPullPolicy)
	}

	clusterPrefixPool := getClusterPrefixPool(s.Status.SliceConfig.SliceSubnet, ipamOctet)

	privileged := true

	sliceRouterContainer := corev1.Container{
		Image:           vl3Image,
		Name:            "vl3-nse",
		ImagePullPolicy: vl3ImagePullPolicy,
		Env: []corev1.EnvVar{
			{
				Name:  "DATAPLANE",
				Value: dataplane,
			},
			{
				Name:  "ENDPOINT_NETWORK_SERVICE",
				Value: "vl3-service-" + s.Name,
			},
			{
				Name:  "ENDPOINT_LABELS",
				Value: "app=" + "vl3-nse-" + s.Name,
			},
			{
				Name:  "TRACER_ENABLED",
				Value: "true",
			},
			{
				Name:  "NSREGISTRY_ADDR",
				Value: "nsmgr." + ControlPlaneNamespace,
			},
			{
				Name:  "NSREGISTRY_PORT",
				Value: "5000",
			},
		},
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				"networkservicemesh.io/socket": *resource.NewQuantity(1, resource.DecimalExponent),
			},
		},
		SecurityContext: &corev1.SecurityContext{
			Privileged: &privileged,
		},
	}

	if dataplane == nsmDataplaneKernel {
		sliceRouterContainer.Env = append(sliceRouterContainer.Env,
			corev1.EnvVar{
				Name:  "IP_ADDRESS",
				Value: clusterPrefixPool,
			},
			corev1.EnvVar{
				Name:  "DST_ROUTES",
				Value: s.Status.SliceConfig.SliceSubnet,
			},
			corev1.EnvVar{
				Name:  "DNS_NAMESERVERS",
				Value: s.Status.DNSIP,
			},
			corev1.EnvVar{
				Name:  "DNS_DOMAINS",
				Value: "slice.local",
			},
		)
	}

	if dataplane == nsmDataplaneVpp {
		sliceRouterContainer.Env = append(sliceRouterContainer.Env,
			corev1.EnvVar{
				Name:  "NSE_IPAM_UNIQUE_OCTET",
				Value: ipamOctet,
			},
		)
		sliceRouterContainer.VolumeMounts = append(sliceRouterContainer.VolumeMounts,
			corev1.VolumeMount{
				Name:      "universal-cnf-config-volume",
				MountPath: "/etc/universal-cnf/config.yaml",
				SubPath:   "config.yaml",
			},
		)
	}

	return sliceRouterContainer
}

func (r *SliceReconciler) getContainerSpecForSliceRouterSidecar(dataplane string) corev1.Container {
	vl3SidecarImage, vl3SidecarImagePullPolicy := getSliceRouterSidecarImageAndPullPolicy()

	privileged := true

	sliceRouterSidecarContainer := corev1.Container{
		Name:            "avesha-vl3-sidecar",
		Image:           vl3SidecarImage,
		ImagePullPolicy: vl3SidecarImagePullPolicy,
		Env: []corev1.EnvVar{
			{
				Name:  "DATAPLANE",
				Value: dataplane,
			},
			{
				Name:  "POD_TYPE",
				Value: "SLICEROUTER_POD",
			},
		},
		SecurityContext: &corev1.SecurityContext{
			Privileged:               &privileged,
			AllowPrivilegeEscalation: &privileged,
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{
					"NET_ADMIN",
				},
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "shared-volume",
				MountPath: "/config",
			},
		},
	}

	return sliceRouterSidecarContainer

}

func (r *SliceReconciler) getVolumeSpecForSliceRouter(s *meshv1beta1.Slice, dataplane string) []corev1.Volume {
	sliceRouterVolumeSpec := []corev1.Volume{{
		Name: "shared-volume",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	},
	}

	if dataplane == nsmDataplaneVpp {
		sliceRouterVolumeSpec = append(sliceRouterVolumeSpec, corev1.Volume{
			Name: "universal-cnf-config-volume",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "ucnf-vl3-service-" + s.Name,
					},
				},
			},
		},
		)
	}

	return sliceRouterVolumeSpec
}

// Creates a deployment spec for the vL3 slice router
func (r *SliceReconciler) deploymentForSliceRouter(s *meshv1beta1.Slice, ipamOctet string, dataplane string) *appsv1.Deployment {
	var replicas int32 = 1

	ls := labelsForSliceRouterDeployment(s.Name)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sliceRouterDeploymentNamePrefix + s.Name,
			Namespace: s.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "nsmgr-acc",
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{{
									MatchExpressions: []corev1.NodeSelectorRequirement{{
										Key:      "avesha/node-type",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"gateway"},
									}},
								}},
							},
						},
					},

					Containers: []corev1.Container{
						r.getContainerSpecForSliceRouter(s, ipamOctet, dataplane),
						r.getContainerSpecForSliceRouterSidecar(dataplane),
					},
					Volumes: r.getVolumeSpecForSliceRouter(s, dataplane),
					Tolerations: []corev1.Toleration{{
						Key:      "avesha/node-type",
						Operator: "Equal",
						Effect:   "NoSchedule",
						Value:    "gateway",
					}, {
						Key:      "avesha/node-type",
						Operator: "Equal",
						Effect:   "NoExecute",
						Value:    "gateway",
					}},
				},
			},
		},
	}

	if len(imagePullSecretName) != 0 {
		dep.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{
			Name: imagePullSecretName,
		}}
	}

	ctrl.SetControllerReference(s, dep, r.Scheme)
	return dep
}

// Deploys the vL3 slice router.
// The configmap needed for the NSE is created first before the NSE is launched.
func (r *SliceReconciler) deploySliceRouter(ctx context.Context, slice *meshv1beta1.Slice) error {
	log := logger.FromContext(ctx).WithName("slice-router")

	dataplane, err := r.getNsmDataplaneMode(ctx, slice)
	if err != nil {
		log.Error(err, "Failed to get nsm dataplane mode. Cannot deploy slice router")
		return err
	}
	if dataplane != nsmDataplaneKernel && dataplane != nsmDataplaneVpp {
		return goerrors.New(fmt.Sprintf("Invalid dataplane: %v", dataplane))
	}

	ipamOctet := strconv.Itoa(slice.Status.SliceConfig.SliceIpam.IpamClusterOctet)

	dep := r.deploymentForSliceRouter(slice, ipamOctet, dataplane)
	err = r.Create(ctx, dep)
	if err != nil {
		log.Error(err, "Failed to create deployment for slice router")
		return err
	}
	log.Info("Created deployment spec for slice router: ", "Name: ", slice.Name, "ipamOctet: ", ipamOctet)

	return nil
}

// Deploys the vL3 slice router service
func (r *SliceReconciler) deploySliceRouterSvc(ctx context.Context, slice *meshv1beta1.Slice) error {
	log := logger.FromContext(ctx).WithName("slice-router-svc")

	ls := labelsForSliceRouterDeployment(slice.Name)

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sliceRouterDeploymentNamePrefix + slice.Name,
			Namespace: ControlPlaneNamespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: ls,
			Ports: []corev1.ServicePort{{
				Port: 5000,
				Name: "grpc",
			}},
		},
	}

	err := r.Create(ctx, svc)
	if err != nil {
		log.Error(err, "Failed to create svc for slice router")
		return err
	}
	log.Info("Created svc spec for slice router: ", "Name: ", slice.Name)

	return nil
}

func (r *SliceReconciler) ReconcileSliceRouter(ctx context.Context, slice *meshv1beta1.Slice) (ctrl.Result, error, bool) {
	log := logger.FromContext(ctx).WithName("slice-router")
	// Spawn the slice router for the slice if not done already
	foundSliceRouter := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: sliceRouterDeploymentNamePrefix + slice.Name, Namespace: slice.Namespace}, foundSliceRouter)
	if err != nil {
		if errors.IsNotFound(err) {
			if slice.Status.SliceConfig == nil {
				return ctrl.Result{
					RequeueAfter: 10 * time.Second,
				}, nil, true
			}
			// Define and create a new deployment for the slice router
			err := r.deploySliceRouter(ctx, slice)
			if err != nil {
				log.Error(err, "Failed to deploy slice router")
				return ctrl.Result{}, err, true
			}
			log.Info("Creating slice router", "Namespace", slice.Namespace, "Name", "vl3-nse-"+slice.Name)
			return ctrl.Result{
				RequeueAfter: 10 * time.Second,
			}, nil, true
		}
		return ctrl.Result{}, err, true
	}

	foundSvc := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{
		Name:      sliceRouterDeploymentNamePrefix + slice.Name,
		Namespace: ControlPlaneNamespace,
	}, foundSvc)

	if err != nil {
		if errors.IsNotFound(err) {
			if slice.Status.SliceConfig == nil {
				return ctrl.Result{
					RequeueAfter: 10 * time.Second,
				}, nil, true
			}
			// Define and create a new deployment for the slice router
			err := r.deploySliceRouterSvc(ctx, slice)
			if err != nil {
				log.Error(err, "Failed to deploy slice router service")
				return ctrl.Result{}, err, true
			}
			log.Info("Creating slice router", "Namespace svc", slice.Namespace, "Name", "vl3-nse-"+slice.Name)
			return ctrl.Result{
				RequeueAfter: 10 * time.Second,
			}, nil, true
		}
		return ctrl.Result{}, err, true
	}

	return ctrl.Result{}, nil, false
}

func FindSliceRouterService(ctx context.Context, c client.Client, sliceName string) (bool, error) {
	vl3NseEpList := &nsmv1alpha1.NetworkServiceEndpointList{}
	opts := []client.ListOption{
		client.InNamespace(ControlPlaneNamespace),
		client.MatchingLabels{"app": "vl3-nse-" + sliceName,
			"networkservicename": "vl3-service-" + sliceName},
	}
	err := c.List(ctx, vl3NseEpList, opts...)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if len(vl3NseEpList.Items) == 0 {
		return false, nil
	}

	return true, nil
}
