package slicegateway

import (
	"testing"

	gwsidecarpb "github.com/kubeslice/gateway-sidecar/pkg/sidecar/sidecarpb"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
)

func TestValidateGatewayPodReadiness(t *testing.T) {
	tests := []struct {
		name    string
		sliceGw *kubeslicev1beta1.SliceGateway
		podName string
		wantErr bool
		errText string
	}{
		{
			name:    "nil slice gateway",
			sliceGw: nil,
			podName: "test-pod",
			wantErr: true,
			errText: "sliceGw is nil",
		},
		{
			name: "pod not found in status",
			sliceGw: &kubeslicev1beta1.SliceGateway{
				Status: kubeslicev1beta1.SliceGatewayStatus{
					GatewayPodStatus: []*kubeslicev1beta1.GwPodInfo{},
				},
			},
			podName: "test-pod",
			wantErr: true,
			errText: "not found in gateway status",
		},
		{
			name: "tunnel not up",
			sliceGw: &kubeslicev1beta1.SliceGateway{
				Status: kubeslicev1beta1.SliceGatewayStatus{
					GatewayPodStatus: []*kubeslicev1beta1.GwPodInfo{
						{
							PodName: "test-pod",
							TunnelStatus: kubeslicev1beta1.TunnelStatus{
								Status: int32(gwsidecarpb.TunnelStatusType_GW_TUNNEL_STATE_DOWN),
							},
						},
					},
				},
			},
			podName: "test-pod",
			wantErr: true,
			errText: "tunnel not up",
		},
		{
			name: "tunnel interface not set",
			sliceGw: &kubeslicev1beta1.SliceGateway{
				Status: kubeslicev1beta1.SliceGatewayStatus{
					GatewayPodStatus: []*kubeslicev1beta1.GwPodInfo{
						{
							PodName: "test-pod",
							TunnelStatus: kubeslicev1beta1.TunnelStatus{
								Status:   int32(gwsidecarpb.TunnelStatusType_GW_TUNNEL_STATE_UP),
								IntfName: "",
							},
						},
					},
				},
			},
			podName: "test-pod",
			wantErr: true,
			errText: "tunnel interface name not set",
		},
		{
			name: "tunnel IPs not configured",
			sliceGw: &kubeslicev1beta1.SliceGateway{
				Status: kubeslicev1beta1.SliceGatewayStatus{
					GatewayPodStatus: []*kubeslicev1beta1.GwPodInfo{
						{
							PodName: "test-pod",
							TunnelStatus: kubeslicev1beta1.TunnelStatus{
								Status:   int32(gwsidecarpb.TunnelStatusType_GW_TUNNEL_STATE_UP),
								IntfName: "tun0",
								LocalIP:  "",
								RemoteIP: "10.0.0.2",
							},
						},
					},
				},
			},
			podName: "test-pod",
			wantErr: true,
			errText: "tunnel IPs not configured",
		},
		{
			name: "peer pod name missing",
			sliceGw: &kubeslicev1beta1.SliceGateway{
				Status: kubeslicev1beta1.SliceGatewayStatus{
					GatewayPodStatus: []*kubeslicev1beta1.GwPodInfo{
						{
							PodName: "test-pod",
							TunnelStatus: kubeslicev1beta1.TunnelStatus{
								Status:   int32(gwsidecarpb.TunnelStatusType_GW_TUNNEL_STATE_UP),
								IntfName: "tun0",
								LocalIP:  "10.0.0.1",
								RemoteIP: "10.0.0.2",
							},
							PeerPodName: "",
						},
					},
				},
			},
			podName: "test-pod",
			wantErr: true,
			errText: "peer pod name not available",
		},
		{
			name: "all checks pass",
			sliceGw: &kubeslicev1beta1.SliceGateway{
				Status: kubeslicev1beta1.SliceGatewayStatus{
					GatewayPodStatus: []*kubeslicev1beta1.GwPodInfo{
						{
							PodName: "test-pod",
							TunnelStatus: kubeslicev1beta1.TunnelStatus{
								Status:   int32(gwsidecarpb.TunnelStatusType_GW_TUNNEL_STATE_UP),
								IntfName: "tun0",
								LocalIP:  "10.0.0.1",
								RemoteIP: "10.0.0.2",
							},
							PeerPodName: "peer-pod",
						},
					},
				},
			},
			podName: "test-pod",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGatewayPodReadiness(tt.sliceGw, tt.podName)

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateGatewayPodReadiness() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				if tt.errText != "" && !stringContains(err.Error(), tt.errText) {
					t.Errorf("ValidateGatewayPodReadiness() error = %v, expected to contain %q", err, tt.errText)
				}
			}
		})
	}
}

// stringContains checks if string s contains substring
func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
