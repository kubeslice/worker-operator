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
		Type:                events.EventTypeNormal,
		ReportingController: "worker",
		Message:             "Slice delted successfully",
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
		Message:             "Slice Router Service failed",
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
		Type:                events.EventTypeNormal,
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
		Message:             "Slice GateWay Connection context has failed.",
	},
	"SliceRouterConnectionContextFailed": {
		Name:                "SliceRouterConnectionContextFailed",
		Reason:              "SliceRouterConnectionContextFailed",
		Action:              "ReconcileSliceGWPod",
		Type:                events.EventTypeWarning,
		ReportingController: "worker",
		Message:             "Slice Router connection context has failed.",
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
		Type:                events.EventTypeNormal,
		ReportingController: "worker",
		Message:             "Slice ServiceImport deleted.",
	},
	"SliceServiceImportDeleteFailed": {
		Name:                "SliceServiceImportDeleteFailed",
		Reason:              "SliceServiceImportDeleteFailed",
		Action:              "DeleteServiceImport",
		Type:                events.EventTypeNormal,
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
		Type:                events.EventTypeWarning,
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
)
