package events

// Autogenerated file. DO NOT MODIFY DIRECTLY!
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

import "github.com/kubeslice/kubeslice-monitoring/pkg/events"

var EventsMap = map[events.EventName]*events.EventSchema{
	"NetPolViolation": {
		Name:                "NetPolViolation",
		Reason:              "PolicyViolation",
		Action:              "PolicyMonitoring",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Network policy violation - please ask admin to check the network policy configuration on the worker cluster. <link to tech doc event-list>",
	},
	"ClusterUnhealthy": {
		Name:                "ClusterUnhealthy",
		Reason:              "ComponentStatusChange",
		Action:              "CheckComponents",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Cluster is unhealthy - Please check if all worker components are running as expected",
	},
	"ClusterHealthy": {
		Name:                "ClusterHealthy",
		Reason:              "ComponentStatusChange",
		Action:              "None",
		Type:                events.EventTypeNormal,
		ReportingController: "worker",
		Message:             "Cluster is healthy - Cluster is back to healthy state",
	},
	"ClusterNodeIpAutoDetected": {
		Name:                "ClusterNodeIpAutoDetected",
		Reason:              "ClusterNodeIpAutoDetected",
		Action:              "None",
		Type:                events.EventTypeNormal,
		ReportingController: "worker",
		Message:             "Auto-detection of cluster node IP addresses was successful due to changes detected in the worker nodes",
	},
	"ClusterNodeIpAutoDetectionFailed": {
		Name:                "ClusterNodeIpAutoDetectionFailed",
		Reason:              "ClusterNodeIpAutoDetectionFailed",
		Action:              "None",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Auto-detection of cluster node IP addresses failed",
	},
	"SliceCreated": {
		Name:                "SliceCreated",
		Reason:              "SliceCreated",
		Action:              "CreateSlice",
		Type:                events.EventTypeNormal,
		ReportingController: "worker",
		Message:             "Slice created successfully",
	},
	"SliceDeleted": {
		Name:                "SliceDeleted",
		Reason:              "SliceDeleted",
		Action:              "DeleteSlice",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Slice deleted successfully",
	},
	"SliceUpdated": {
		Name:                "SliceUpdated",
		Reason:              "SliceUpdated",
		Action:              "UpdateSlice",
		Type:                events.EventTypeNormal,
		ReportingController: "worker",
		Message:             "Slice updated successfully",
	},
	"SliceCreationFailed": {
		Name:                "SliceCreationFailed",
		Reason:              "SliceCreationFailed",
		Action:              "CreateSlice",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Slice creation failed, please check the slice configuration.",
	},
	"SliceDeletionFailed": {
		Name:                "SliceDeletionFailed",
		Reason:              "SliceDeletionFailed",
		Action:              "DeleteSlice",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Slice deletion failed, please check the slice status",
	},
	"SliceUpdateFailed": {
		Name:                "SliceUpdateFailed",
		Reason:              "SliceUpdateFailed",
		Action:              "UpdateSlice",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Slice update failed",
	},
	"SliceQoSProfileWithNetOpsSync": {
		Name:                "SliceQoSProfileWithNetOpsSync",
		Reason:              "SliceQoSProfileWithNetOpsSyncFailed",
		Action:              "ReconcileSlice",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Slice QoS Sync with NetOp failed - please ask admin to check the Slice QoS Profile",
	},
	"SliceIngressInstallFailed": {
		Name:                "SliceIngressInstallFailed",
		Reason:              "SliceIngressInstallFailed",
		Action:              "ReconcileSlice",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "slice ingress installation failed",
	},
	"SliceEgressInstallFailed": {
		Name:                "SliceEgressInstallFailed",
		Reason:              "SliceEgressInstallFailed",
		Action:              "ReconcileSlice",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "slice egress installation failed",
	},
	"SliceAppPodsListUpdateFailed": {
		Name:                "SliceAppPodsListUpdateFailed",
		Reason:              "SliceAppPodsListUpdateFailed",
		Action:              "ReconcileSlice",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "slice app pods list is not updated - please ask admin to check slice configuration.",
	},
	"SliceRouterDeploymentFailed": {
		Name:                "SliceRouterDeploymentFailed",
		Reason:              "SliceRouterDeploymentFailed",
		Action:              "ReconcileSlice",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "slice Router Deployment failed.",
	},
	"SliceRouterServiceFailed": {
		Name:                "SliceRouterServiceFailed",
		Reason:              "SliceRouterServiceFailed",
		Action:              "ReconcileSlice",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Failed to create Service for slice router.",
	},
	"WorkerSliceConfigUpdated": {
		Name:                "WorkerSliceConfigUpdated",
		Reason:              "WorkerSliceConfigUpdated",
		Action:              "ReconcileWorkerSliceConfig",
		Type:                events.EventTypeNormal,
		ReportingController: "worker",
		Message:             "WorkerSliceConfig updated.",
	},
	"WorkerSliceHealthUpdated": {
		Name:                "WorkerSliceHealthUpdated",
		Reason:              "WorkerSliceHealthUpdated",
		Action:              "ReconcileWorkerSliceConfig",
		Type:                events.EventTypeNormal,
		ReportingController: "worker",
		Message:             "WorkerSliceHealth updated.",
	},
	"WorkerSliceHealthUpdateFailed": {
		Name:                "WorkerSliceHealthUpdateFailed",
		Reason:              "WorkerSliceHealthUpdateFailed",
		Action:              "ReconcileWorkerSliceConfig",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "WorkerSliceHealth update failed.",
	},
	"SliceGWCreated": {
		Name:                "SliceGWCreated",
		Reason:              "SliceGWCreated",
		Action:              "CreateSliceGW",
		Type:                events.EventTypeNormal,
		ReportingController: "worker",
		Message:             "Slice GateWay created.",
	},
	"SliceGWDeleted": {
		Name:                "SliceGWDeleted",
		Reason:              "SliceGWDeleted",
		Action:              "DeleteSliceGW",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Slice GateWay deleted.",
	},
	"SliceGWUpdated": {
		Name:                "SliceGWUpdated",
		Reason:              "SliceGWUpdated",
		Action:              "UpdateSliceGW",
		Type:                events.EventTypeNormal,
		ReportingController: "worker",
		Message:             "Slice GateWay updated.",
	},
	"SliceGWCreateFailed": {
		Name:                "SliceGWCreateFailed",
		Reason:              "SliceGWCreateFailed",
		Action:              "CreateSliceGW",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Slice GateWay creation failed.",
	},
	"SliceGWDeleteFailed": {
		Name:                "SliceGWDeleteFailed",
		Reason:              "SliceGWDeleteFailed",
		Action:              "DeleteSliceGW",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Slice GateWay delete failed.",
	},
	"SliceGWUpdateFailed": {
		Name:                "SliceGWUpdateFailed",
		Reason:              "SliceGWUpdateFailed",
		Action:              "UpdateSliceGW",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Slice GateWay update failed.",
	},
	"SliceGWPodReconcileFailed": {
		Name:                "SliceGWPodReconcileFailed",
		Reason:              "SliceGWPodReconcileFailed",
		Action:              "ReconcileSliceGWPod",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Slice GateWay pod reconcilation failed.",
	},
	"SliceGWConnectionContextFailed": {
		Name:                "SliceGWConnectionContextFailed",
		Reason:              "SliceGWConnectionContextFailed",
		Action:              "ReconcileSliceGWPod",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Failed to update the connection context on slice GateWay pods.",
	},
	"SliceRouterConnectionContextFailed": {
		Name:                "SliceRouterConnectionContextFailed",
		Reason:              "SliceRouterConnectionContextFailed",
		Action:              "ReconcileSliceGWPod",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Failed to send connection context to slice router pod.",
	},
	"SliceNetopQoSSyncFailed": {
		Name:                "SliceNetopQoSSyncFailed",
		Reason:              "SliceNetopQoSSyncFailed",
		Action:              "ReconcileSliceGWPod",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Slice NetOp QoS sync failed.",
	},
	"SliceGWRebalancingFailed": {
		Name:                "SliceGWRebalancingFailed",
		Reason:              "SliceGWRebalancingFailed",
		Action:              "ReconcileSliceGWPod",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Slice GateWay rebalancing failed.",
	},
	"SliceGWRemotePodSyncFailed": {
		Name:                "SliceGWRemotePodSyncFailed",
		Reason:              "SliceGWRemotePodSyncFailed",
		Action:              "ReconcileSliceGWPod",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Slice GateWay Remote pod sync failed.",
	},
	"SliceGWRebalancingSuccess": {
		Name:                "SliceGWRebalancingSuccess",
		Reason:              "SliceGWRebalancingSuccess",
		Action:              "ReconcileSliceGWPod",
		Type:                events.EventTypeNormal,
		ReportingController: "worker",
		Message:             "Slice GateWay rebalancing successfully.",
	},
	"SliceGWServiceCreationFailed": {
		Name:                "SliceGWServiceCreationFailed",
		Reason:              "SliceGWServiceCreationFailed",
		Action:              "ReconcileSliceGWPod",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Slice GateWay Service creation failed.",
	},
	"SliceGWNodePortUpdateFailed": {
		Name:                "SliceGWNodePortUpdateFailed",
		Reason:              "SliceGWNodePortUpdateFailed",
		Action:              "ReconcileSliceGWPod",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Slice GateWay NodePort update failed.",
	},
	"SliceServiceImportUpdateAvailableEndpointsFailed": {
		Name:                "SliceServiceImportUpdateAvailableEndpointsFailed",
		Reason:              "SliceServiceImportUpdateAvailableEndpointsFailed",
		Action:              "ReconcileServiceImport",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Slice ServiceImport Available endpoints update has failed.",
	},
	"SliceServiceImportDeleted": {
		Name:                "SliceServiceImportDeleted",
		Reason:              "SliceServiceImportDeleted",
		Action:              "DeleteServiceImport",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Slice ServiceImport deleted.",
	},
	"SliceServiceImportDeleteFailed": {
		Name:                "SliceServiceImportDeleteFailed",
		Reason:              "SliceServiceImportDeleteFailed",
		Action:              "DeleteServiceImport",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Slice ServiceImport deletion failed.",
	},
	"SliceServiceImportUpdatePorts": {
		Name:                "SliceServiceImportUpdatePorts",
		Reason:              "SliceServiceImportUpdatePorts",
		Action:              "ReconcileServiceImport",
		Type:                events.EventTypeNormal,
		ReportingController: "worker",
		Message:             "Slice ServiceImport ports updated.",
	},
	"SliceServiceImportUpdatePortsFailed": {
		Name:                "SliceServiceImportUpdatePortsFailed",
		Reason:              "SliceServiceImportUpdatePortsFailed",
		Action:              "ReconcileServiceImport",
		Type:                events.EventTypeNormal,
		ReportingController: "worker",
		Message:             "Slice ServiceImport ports update failed.",
	},
	"SliceServiceImportUpdated": {
		Name:                "SliceServiceImportUpdated",
		Reason:              "SliceServiceImportUpdated",
		Action:              "ReconcileServiceImport",
		Type:                events.EventTypeNormal,
		ReportingController: "worker",
		Message:             "Slice ServiceImport updated.",
	},
	"SliceServiceImportUpdateFailed": {
		Name:                "SliceServiceImportUpdateFailed",
		Reason:              "SliceServiceImportUpdateFailed",
		Action:              "ReconcileServiceImport",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Slice ServiceImport update failed.",
	},
	"NetPolAdded": {
		Name:                "NetPolAdded",
		Reason:              "NetPolAdded",
		Action:              "ReconcileNetPol",
		Type:                events.EventTypeNormal,
		ReportingController: "worker",
		Message:             "Slice Network policy added.",
	},
	"NetPolScopeWidenedNamespace": {
		Name:                "NetPolScopeWidenedNamespace",
		Reason:              "Scope widened with reason - Namespace violation",
		Action:              "ReconcileNetPol",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Slice NetworkPolicy scope widened due to namespace violation.",
	},
	"NetPolScopeWidenedIPBlock": {
		Name:                "NetPolScopeWidenedIPBlock",
		Reason:              "Scope widened with reason - IPBlock violation",
		Action:              "ReconcileNetPol",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Slice NetworkPolicy scope widened due to IP Block violation.",
	},
	"WorkerServiceImportCreateFailed": {
		Name:                "WorkerServiceImportCreateFailed",
		Reason:              "WorkerServiceImportCreateFailed",
		Action:              "CreateWorkerServiceImport",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Worker ServiceImport Creation failed.",
	},
	"WorkerServiceImportCreated": {
		Name:                "WorkerServiceImportCreated",
		Reason:              "WorkerServiceImportCreated",
		Action:              "CreateWorkerServiceImport",
		Type:                events.EventTypeNormal,
		ReportingController: "worker",
		Message:             "Worker ServiceImport created.",
	},
	"ServiceExportDeleted": {
		Name:                "ServiceExportDeleted",
		Reason:              "ServiceExportDeleted",
		Action:              "DeleteServiceExport",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "ServiceExport deleted.",
	},
	"ServiceExportDeleteFailed": {
		Name:                "ServiceExportDeleteFailed",
		Reason:              "ServiceExportDeleteFailed",
		Action:              "DeleteServiceExport",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "ServiceExport delete failed.",
	},
	"ServiceExportInitialStatusUpdated": {
		Name:                "ServiceExportInitialStatusUpdated",
		Reason:              "ServiceExportInitialStatusUpdated",
		Action:              "ReconcileServiceExport",
		Type:                events.EventTypeNormal,
		ReportingController: "worker",
		Message:             "ServiceExport initial status updated.",
	},
	"SliceServiceExportInitialStatusUpdateFailed": {
		Name:                "SliceServiceExportInitialStatusUpdateFailed",
		Reason:              "SliceServiceExportInitialStatusUpdateFailed",
		Action:              "ReconcileServiceExport",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "ServiceExport initial status update failed.",
	},
	"ServiceExportSliceFetchFailed": {
		Name:                "ServiceExportSliceFetchFailed",
		Reason:              "ServiceExportSliceFetchFailed",
		Action:              "ReconcileServiceExport",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "ServiceExport slice fetch failed.",
	},
	"ServiceExportStatusPending": {
		Name:                "ServiceExportStatusPending",
		Reason:              "ServiceExportStatusPending",
		Action:              "ReconcileServiceExport",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "ServiceExport status pending.",
	},
	"ServiceExportUpdatePortsFailed": {
		Name:                "ServiceExportUpdatePortsFailed",
		Reason:              "ServiceExportUpdatePortsFailed",
		Action:              "ReconcileServiceExport",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "ServiceExport update of ports failed.",
	},
	"IngressGWPodReconciledSuccessfully": {
		Name:                "IngressGWPodReconciledSuccessfully",
		Reason:              "IngressGWPodReconciledSuccessfully",
		Action:              "ReconcileServiceExport",
		Type:                events.EventTypeNormal,
		ReportingController: "worker",
		Message:             "Ingress GateWay pod reconciled.",
	},
	"IngressGWPodReconcileFailed": {
		Name:                "IngressGWPodReconcileFailed",
		Reason:              "IngressGWPodReconcileFailed",
		Action:              "ReconcileServiceExport",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Ingress GateWay pod reconcilation failed.",
	},
	"SyncServiceExportStatusFailed": {
		Name:                "SyncServiceExportStatusFailed",
		Reason:              "SyncServiceExportStatusFailed",
		Action:              "ReconcileServiceExport",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "ServiceExport status sync failed.",
	},
	"SyncServiceExportStatusSuccessfully": {
		Name:                "SyncServiceExportStatusSuccessfully",
		Reason:              "SyncServiceExportStatusSuccessfully",
		Action:              "ReconcileServiceExport",
		Type:                events.EventTypeNormal,
		ReportingController: "worker",
		Message:             "ServiceExport status sync successful.",
	},
	"FSMNewGWSpawned": {
		Name:                "FSMNewGWSpawned",
		Reason:              "FSMNewGWSpawned",
		Action:              "ReconcileWorkerSliceGWRecycler",
		Type:                events.EventTypeNormal,
		ReportingController: "worker",
		Message:             "sliceGWRecyler - New GateWay spawned.",
	},
	"FSMRoutingTableUpdated": {
		Name:                "FSMRoutingTableUpdated",
		Reason:              "FSMRoutingTableUpdated",
		Action:              "ReconcileWorkerSliceGWRecycler",
		Type:                events.EventTypeNormal,
		ReportingController: "worker",
		Message:             "sliceGWRecyler - Routing table updated.",
	},
	"FSMDeleteOldGW": {
		Name:                "FSMDeleteOldGW",
		Reason:              "FSMDeleteOldGW",
		Action:              "ReconcileWorkerSliceGWRecycler",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "sliceGWRecyler - old GateWay deleted.",
	},
	"FSMNewGWSpawnFailed": {
		Name:                "FSMNewGWSpawnFailed",
		Reason:              "FSMNewGWSpawnFailed",
		Action:              "ReconcileWorkerSliceGWRecycler",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "sliceGWRecyler - new GateWay failed to spawn up.",
	},
	"FSMRoutingTableUpdateFailed": {
		Name:                "FSMRoutingTableUpdateFailed",
		Reason:              "FSMRoutingTableUpdateFailed",
		Action:              "ReconcileWorkerSliceGWRecycler",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "sliceGWRecyler - Routing table update failed.",
	},
	"FSMDeleteOldGWFailed": {
		Name:                "FSMDeleteOldGWFailed",
		Reason:              "FSMDeleteOldGWFailed",
		Action:              "ReconcileWorkerSliceGWRecycler",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "sliceGWRecyler - failed to delete old GateWay.",
	},
	"ClusterProviderUpdateInfoSuccesful": {
		Name:                "ClusterProviderUpdateInfoSuccesful",
		Reason:              "ClusterProviderUpdateInfoSuccesful",
		Action:              "UpdateCluster",
		Type:                events.EventTypeNormal,
		ReportingController: "worker",
		Message:             "Cluster provider info updated successfully.",
	},
	"ClusterProviderUpdateInfoFailed": {
		Name:                "ClusterProviderUpdateInfoFailed",
		Reason:              "ClusterProviderUpdateInfoFailed",
		Action:              "ReconcileCluster",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "cluster provider info update failed.",
	},
	"ClusterCNISubnetUpdateSuccessful": {
		Name:                "ClusterCNISubnetUpdateSuccessful",
		Reason:              "ClusterCNISubnetUpdateSuccessful",
		Action:              "UpdateCluster",
		Type:                events.EventTypeNormal,
		ReportingController: "worker",
		Message:             "cluster CNI Subnet updated successfully.",
	},
	"ClusterCNISubnetUpdateFailed": {
		Name:                "ClusterCNISubnetUpdateFailed",
		Reason:              "ClusterCNISubnetUpdateFailed",
		Action:              "ReconcileCluster",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "cluster CNI Subnet update failed.",
	},
	"ClusterNodeIpUpdateSuccessful": {
		Name:                "ClusterNodeIpUpdateSuccessful",
		Reason:              "ClusterNodeIpUpdateSuccessful",
		Action:              "UpdateCluster",
		Type:                events.EventTypeNormal,
		ReportingController: "worker",
		Message:             "cluster nodeIP updated successfully.",
	},
	"ClusterNodeIpUpdateFailed": {
		Name:                "ClusterNodeIpUpdateFailed",
		Reason:              "ClusterNodeIpUpdateFailed",
		Action:              "ReconcileCluster",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "cluster nodeIP update failed.",
	},
	"ClusterDashboardCredsUpdated": {
		Name:                "ClusterDashboardCredsUpdated",
		Reason:              "ClusterDashboardCredsUpdated",
		Action:              "UpdateCluster",
		Type:                events.EventTypeNormal,
		ReportingController: "worker",
		Message:             "cluster dashboard credentials updated successfully.",
	},
	"ClusterDashboardCredsUpdateFailed": {
		Name:                "ClusterDashboardCredsUpdateFailed",
		Reason:              "ClusterDashboardCredsUpdateFailed",
		Action:              "ReconcileCluster",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "cluster dashboard credentails update failed.",
	},
	"ClusterHealthStatusUpdated": {
		Name:                "ClusterHealthStatusUpdated",
		Reason:              "ClusterHealthStatusUpdated",
		Action:              "UpdateCluster",
		Type:                events.EventTypeNormal,
		ReportingController: "worker",
		Message:             "cluster health status updated.",
	},
	"ClusterHealthStatusUpdateFailed": {
		Name:                "ClusterHealthStatusUpdateFailed",
		Reason:              "ClusterHealthStatusUpdateFailed",
		Action:              "ReconcileCluster",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "cluster health status update failed.",
	},
	"UpdatedNamespaceInfoToHub": {
		Name:                "UpdatedNamespaceInfoToHub",
		Reason:              "UpdatedNamespaceInfoToHub",
		Action:              "ReconcileNamespace",
		Type:                events.EventTypeNormal,
		ReportingController: "worker",
		Message:             "namespace info updated to Controller.",
	},
	"UpdateNamespaceInfoToHubFailed": {
		Name:                "UpdateNamespaceInfoToHubFailed",
		Reason:              "UpdateNamespaceInfoToHubFailed",
		Action:              "ReconcileNamespace",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "namespace info failed to update to Controller.",
	},
	"DeleteNamespaceInfoToHub": {
		Name:                "DeleteNamespaceInfoToHub",
		Reason:              "DeleteNamespaceInfoToHub",
		Action:              "ReconcileNamespace",
		Type:                events.EventTypeNormal,
		ReportingController: "worker",
		Message:             "namespace info deleted from Controller.",
	},
	"DeleteNamespaceInfoToHubFailed": {
		Name:                "DeleteNamespaceInfoToHubFailed",
		Reason:              "DeleteNamespaceInfoToHubFailed",
		Action:              "ReconcileNamespace",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "namespace info failed to be deleted from Controller.",
	},
	"DeregistrationJobFailed": {
		Name:                "DeregistrationJobFailed",
		Reason:              "DeregistrationJobFailed",
		Action:              "None",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Failed to complete cluster deregistration job",
	},
}

var (
	EventNetPolViolation                                  events.EventName = "NetPolViolation"
	EventClusterUnhealthy                                 events.EventName = "ClusterUnhealthy"
	EventClusterHealthy                                   events.EventName = "ClusterHealthy"
	EventClusterNodeIpAutoDetected                        events.EventName = "ClusterNodeIpAutoDetected"
	EventClusterNodeIpAutoDetectionFailed                 events.EventName = "ClusterNodeIpAutoDetectionFailed"
	EventSliceCreated                                     events.EventName = "SliceCreated"
	EventSliceDeleted                                     events.EventName = "SliceDeleted"
	EventSliceUpdated                                     events.EventName = "SliceUpdated"
	EventSliceCreationFailed                              events.EventName = "SliceCreationFailed"
	EventSliceDeletionFailed                              events.EventName = "SliceDeletionFailed"
	EventSliceUpdateFailed                                events.EventName = "SliceUpdateFailed"
	EventSliceQoSProfileWithNetOpsSync                    events.EventName = "SliceQoSProfileWithNetOpsSync"
	EventSliceIngressInstallFailed                        events.EventName = "SliceIngressInstallFailed"
	EventSliceEgressInstallFailed                         events.EventName = "SliceEgressInstallFailed"
	EventSliceAppPodsListUpdateFailed                     events.EventName = "SliceAppPodsListUpdateFailed"
	EventSliceRouterDeploymentFailed                      events.EventName = "SliceRouterDeploymentFailed"
	EventSliceRouterServiceFailed                         events.EventName = "SliceRouterServiceFailed"
	EventWorkerSliceConfigUpdated                         events.EventName = "WorkerSliceConfigUpdated"
	EventWorkerSliceHealthUpdated                         events.EventName = "WorkerSliceHealthUpdated"
	EventWorkerSliceHealthUpdateFailed                    events.EventName = "WorkerSliceHealthUpdateFailed"
	EventSliceGWCreated                                   events.EventName = "SliceGWCreated"
	EventSliceGWDeleted                                   events.EventName = "SliceGWDeleted"
	EventSliceGWUpdated                                   events.EventName = "SliceGWUpdated"
	EventSliceGWCreateFailed                              events.EventName = "SliceGWCreateFailed"
	EventSliceGWDeleteFailed                              events.EventName = "SliceGWDeleteFailed"
	EventSliceGWUpdateFailed                              events.EventName = "SliceGWUpdateFailed"
	EventSliceGWPodReconcileFailed                        events.EventName = "SliceGWPodReconcileFailed"
	EventSliceGWConnectionContextFailed                   events.EventName = "SliceGWConnectionContextFailed"
	EventSliceRouterConnectionContextFailed               events.EventName = "SliceRouterConnectionContextFailed"
	EventSliceNetopQoSSyncFailed                          events.EventName = "SliceNetopQoSSyncFailed"
	EventSliceGWRebalancingFailed                         events.EventName = "SliceGWRebalancingFailed"
	EventSliceGWRemotePodSyncFailed                       events.EventName = "SliceGWRemotePodSyncFailed"
	EventSliceGWRebalancingSuccess                        events.EventName = "SliceGWRebalancingSuccess"
	EventSliceGWServiceCreationFailed                     events.EventName = "SliceGWServiceCreationFailed"
	EventSliceGWNodePortUpdateFailed                      events.EventName = "SliceGWNodePortUpdateFailed"
	EventSliceServiceImportUpdateAvailableEndpointsFailed events.EventName = "SliceServiceImportUpdateAvailableEndpointsFailed"
	EventSliceServiceImportDeleted                        events.EventName = "SliceServiceImportDeleted"
	EventSliceServiceImportDeleteFailed                   events.EventName = "SliceServiceImportDeleteFailed"
	EventSliceServiceImportUpdatePorts                    events.EventName = "SliceServiceImportUpdatePorts"
	EventSliceServiceImportUpdatePortsFailed              events.EventName = "SliceServiceImportUpdatePortsFailed"
	EventSliceServiceImportUpdated                        events.EventName = "SliceServiceImportUpdated"
	EventSliceServiceImportUpdateFailed                   events.EventName = "SliceServiceImportUpdateFailed"
	EventNetPolAdded                                      events.EventName = "NetPolAdded"
	EventNetPolScopeWidenedNamespace                      events.EventName = "NetPolScopeWidenedNamespace"
	EventNetPolScopeWidenedIPBlock                        events.EventName = "NetPolScopeWidenedIPBlock"
	EventWorkerServiceImportCreateFailed                  events.EventName = "WorkerServiceImportCreateFailed"
	EventWorkerServiceImportCreated                       events.EventName = "WorkerServiceImportCreated"
	EventServiceExportDeleted                             events.EventName = "ServiceExportDeleted"
	EventServiceExportDeleteFailed                        events.EventName = "ServiceExportDeleteFailed"
	EventServiceExportInitialStatusUpdated                events.EventName = "ServiceExportInitialStatusUpdated"
	EventSliceServiceExportInitialStatusUpdateFailed      events.EventName = "SliceServiceExportInitialStatusUpdateFailed"
	EventServiceExportSliceFetchFailed                    events.EventName = "ServiceExportSliceFetchFailed"
	EventServiceExportStatusPending                       events.EventName = "ServiceExportStatusPending"
	EventServiceExportUpdatePortsFailed                   events.EventName = "ServiceExportUpdatePortsFailed"
	EventIngressGWPodReconciledSuccessfully               events.EventName = "IngressGWPodReconciledSuccessfully"
	EventIngressGWPodReconcileFailed                      events.EventName = "IngressGWPodReconcileFailed"
	EventSyncServiceExportStatusFailed                    events.EventName = "SyncServiceExportStatusFailed"
	EventSyncServiceExportStatusSuccessfully              events.EventName = "SyncServiceExportStatusSuccessfully"
	EventFSMNewGWSpawned                                  events.EventName = "FSMNewGWSpawned"
	EventFSMRoutingTableUpdated                           events.EventName = "FSMRoutingTableUpdated"
	EventFSMDeleteOldGW                                   events.EventName = "FSMDeleteOldGW"
	EventFSMNewGWSpawnFailed                              events.EventName = "FSMNewGWSpawnFailed"
	EventFSMRoutingTableUpdateFailed                      events.EventName = "FSMRoutingTableUpdateFailed"
	EventFSMDeleteOldGWFailed                             events.EventName = "FSMDeleteOldGWFailed"
	EventClusterProviderUpdateInfoSuccesful               events.EventName = "ClusterProviderUpdateInfoSuccesful"
	EventClusterProviderUpdateInfoFailed                  events.EventName = "ClusterProviderUpdateInfoFailed"
	EventClusterCNISubnetUpdateSuccessful                 events.EventName = "ClusterCNISubnetUpdateSuccessful"
	EventClusterCNISubnetUpdateFailed                     events.EventName = "ClusterCNISubnetUpdateFailed"
	EventClusterNodeIpUpdateSuccessful                    events.EventName = "ClusterNodeIpUpdateSuccessful"
	EventClusterNodeIpUpdateFailed                        events.EventName = "ClusterNodeIpUpdateFailed"
	EventClusterDashboardCredsUpdated                     events.EventName = "ClusterDashboardCredsUpdated"
	EventClusterDashboardCredsUpdateFailed                events.EventName = "ClusterDashboardCredsUpdateFailed"
	EventClusterHealthStatusUpdated                       events.EventName = "ClusterHealthStatusUpdated"
	EventClusterHealthStatusUpdateFailed                  events.EventName = "ClusterHealthStatusUpdateFailed"
	EventUpdatedNamespaceInfoToHub                        events.EventName = "UpdatedNamespaceInfoToHub"
	EventUpdateNamespaceInfoToHubFailed                   events.EventName = "UpdateNamespaceInfoToHubFailed"
	EventDeleteNamespaceInfoToHub                         events.EventName = "DeleteNamespaceInfoToHub"
	EventDeleteNamespaceInfoToHubFailed                   events.EventName = "DeleteNamespaceInfoToHubFailed"
	EventDeregistrationJobFailed                          events.EventName = "DeregistrationJobFailed"
)
