package hub

import (
	"context"
	"fmt"
	"os"
	"strings"

	"bitbucket.org/realtimeai/kubeslice-operator/pkg/kube"

	"k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	log "sigs.k8s.io/controller-runtime/pkg/log"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	"bitbucket.org/realtimeai/kubeslice-operator/internal/cluster"
	"bitbucket.org/realtimeai/kubeslice-operator/internal/logger"
	hubv1alpha1 "bitbucket.org/realtimeai/mesh-apis/pkg/hub/v1alpha1"
	spokev1alpha1 "bitbucket.org/realtimeai/mesh-apis/pkg/spoke/v1alpha1"
)

var scheme = runtime.NewScheme()
var hubClient client.Client

func init() {
	log.SetLogger(logger.NewLogger())
	clientgoscheme.AddToScheme(scheme)
	utilruntime.Must(spokev1alpha1.AddToScheme(scheme))
	utilruntime.Must(hubv1alpha1.AddToScheme(scheme))
	utilruntime.Must(meshv1beta1.AddToScheme(scheme))
	hubClient, _ = client.New(&rest.Config{
		Host:            os.Getenv("HUB_HOST_ENDPOINT"),
		BearerTokenFile: HubTokenFile,
		TLSClientConfig: rest.TLSClientConfig{
			CAFile: HubCAFile,
		}},
		client.Options{
			Scheme: scheme,
		},
	)
}

func UpdateNodePortForSliceGwServer(ctx context.Context, sliceGwNodePort int32, sliceGwName string) error {
	sliceGw := &spokev1alpha1.SpokeSliceGateway{}
	err := hubClient.Get(ctx, types.NamespacedName{
		Name:      sliceGwName,
		Namespace: ProjectNamespace,
	}, sliceGw)
	if err != nil {
		return err
	}

	if sliceGw.Spec.LocalGatewayConfig.NodePort == int(sliceGwNodePort) {
		// No update needed
		return nil
	}

	sliceGw.Spec.LocalGatewayConfig.NodePort = int(sliceGwNodePort)

	return hubClient.Update(ctx, sliceGw)
}

func UpdateClusterInfoToHub(ctx context.Context, clusterName, nodeIP string) error {
	if hubClient == nil{
		return fmt.Errorf("hubClient is nil")
	}
	hubCluster := &hubv1alpha1.Cluster{}
	err := hubClient.Get(ctx, types.NamespacedName{
		Name:      clusterName,
		Namespace: ProjectNamespace,
	}, hubCluster)
	if err != nil {
		return err
	}

	clientset, err := kube.NewClient()
	if err != nil {
		return err
	}

	kubeClient := clientset.KubeCli
	c := cluster.NewCluster(kubeClient, clusterName)
	//get geographical info
	clusterInfo, err := c.GetClusterInfo(ctx)
	if err != nil {
		return err
	}
	cniSubnet, err := c.GetNsmExcludedPrefix(ctx, "nsm-config", "kubeslice-system")
	if err != nil {
		return err
	}
	hubCluster.Spec.ClusterProperty.GeoLocation.CloudRegion = clusterInfo.ClusterProperty.GeoLocation.CloudRegion

	hubCluster.Spec.ClusterProperty.GeoLocation.CloudProvider = clusterInfo.ClusterProperty.GeoLocation.CloudProvider

	hubCluster.Spec.NodeIP = nodeIP

	hubCluster.Spec.CniSubnet = strings.Join(cniSubnet, ",")


	return hubClient.Update(ctx, hubCluster)
}

	
func getHubServiceDiscoveryEps(serviceexport *meshv1beta1.ServiceExport) []hubv1alpha1.ServiceDiscoveryEndpoint {
	epList := []hubv1alpha1.ServiceDiscoveryEndpoint{}
	for _, pod := range serviceexport.Status.Pods {
		ep := hubv1alpha1.ServiceDiscoveryEndpoint{
			PodName: pod.Name,
			Cluster: ClusterName,
			NsmIp:   pod.NsmIP,
			DnsName: pod.DNSName,
		}
		epList = append(epList, ep)
	}

	return epList
}

func getHubServiceDiscoveryPorts(serviceexport *meshv1beta1.ServiceExport) []hubv1alpha1.ServiceDiscoveryPort {
	portList := []hubv1alpha1.ServiceDiscoveryPort{}
	for _, port := range serviceexport.Spec.Ports {
		portList = append(portList, hubv1alpha1.ServiceDiscoveryPort{
			Name:     port.Name,
			Port:     port.ContainerPort,
			Protocol: string(port.Protocol),
		})
	}

	return portList
}

func getHubServiceExportObj(serviceexport *meshv1beta1.ServiceExport) *hubv1alpha1.ServiceExportConfig {
	return &hubv1alpha1.ServiceExportConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceexport.Name,
			Namespace: ProjectNamespace,
		},
		Spec: hubv1alpha1.ServiceExportConfigSpec{
			ServiceName:               serviceexport.Name,
			ServiceNamespace:          serviceexport.ObjectMeta.Namespace,
			SourceCluster:             ClusterName,
			SliceName:                 serviceexport.Spec.Slice,
			MeshType:                  string(serviceexport.Spec.MeshType),
			ServiceDiscoveryEndpoints: getHubServiceDiscoveryEps(serviceexport),
			ServiceDiscoveryPorts:     getHubServiceDiscoveryPorts(serviceexport),
		},
	}
}

func UpdateServiceExport(ctx context.Context, serviceexport *meshv1beta1.ServiceExport) error {
	hubSvcEx := &hubv1alpha1.ServiceExportConfig{}
	err := hubClient.Get(ctx, types.NamespacedName{
		Name:      serviceexport.Name,
		Namespace: ProjectNamespace,
	}, hubSvcEx)
	if err != nil {
		if errors.IsNotFound(err) {
			err = hubClient.Create(ctx, getHubServiceExportObj(serviceexport))
			if err != nil {
				return err
			}
			return nil
		}
		return err
	}

	hubSvcEx.Spec = getHubServiceExportObj(serviceexport).Spec

	err = hubClient.Update(ctx, hubSvcEx)
	if err != nil {
		return err
	}

	return nil
}
