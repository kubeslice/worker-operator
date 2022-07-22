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
package slice

import (
	"context"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers"
	"github.com/kubeslice/worker-operator/pkg/logger"
	webhook "github.com/kubeslice/worker-operator/pkg/webhook/deploy"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type nsMarker struct {
	ns     *corev1.Namespace
	marked bool
}

var (
	allowedNamespacesByDefault = []string{"kubeslice-system", "kube-system", "istio-system"}
)

func (r *SliceReconciler) ReconcileSliceNamespaces(ctx context.Context, slice *kubeslicev1beta1.Slice) (ctrl.Result, error, bool) {
	res, err, reconcile := r.reconcileAppNamespaces(ctx, slice)
	if reconcile {
		return res, err, true
	}
	err = r.reconcileAllowedNamespaces(ctx, slice)
	if err != nil {
		return ctrl.Result{}, err, true
	}
	err = r.reconcileSliceNetworkPolicy(ctx, slice)
	if err != nil {
		return ctrl.Result{}, err, true
	}
	return ctrl.Result{}, nil, false
}

func (r *SliceReconciler) reconcileAppNamespaces(ctx context.Context, slice *kubeslicev1beta1.Slice) (ctrl.Result, error, bool) {
	log := logger.FromContext(ctx).WithValues("type", "appNamespaces")
	debugLog := log.V(1)
	//early exit if NamespaceIsolationProfile is not defined
	if slice.Status.SliceConfig.NamespaceIsolationProfile == nil {
		return ctrl.Result{}, nil, false
	}
	//cfgAppNsList = list of all app namespaces in slice CR
	//var cfgAppNsList []string
	cfgAppNsList := buildAppNamespacesList(slice)
	debugLog.Info("reconciling", "applicationNamespaces", cfgAppNsList)

	// Get the list of existing namespaces that are tagged with the kubeslice label
	// existingAppNsList = list of all namespaces that have kubeslice label
	existingAppNsList := &corev1.NamespaceList{}
	listOpts := []client.ListOption{
		client.MatchingLabels(map[string]string{
			controllers.ApplicationNamespaceSelectorLabelKey: slice.Name,
		}),
	}
	err := r.List(ctx, existingAppNsList, listOpts...)
	if err != nil {
		log.Error(err, "Failed to list namespaces")
		return ctrl.Result{}, err, true
	}
	log.Info("reconciling", "existingAppNsList", existingAppNsList)
	// Convert the list into a map for faster lookups. Will come in handy when we compare
	// existing namespaces against configured namespaces.
	existingAppNsMap := make(map[string]*nsMarker)

	for _, existingAppNsObj := range existingAppNsList.Items {
		existingAppNsMap[existingAppNsObj.ObjectMeta.Name] = &nsMarker{
			ns:     &existingAppNsObj,
			marked: false,
		}
	}
	// Compare the existing list with the configured list.
	// If a namespace is not found in the existing list, consider it as an addition event and
	// label the namespace.
	// If a namespace is found in the existing list, mark it to indicate that it has been verified
	// to be valid as it is present in the configured list as well.
	labeledAppNsList, statusChanged, err := r.labelAppNamespaces(ctx, cfgAppNsList, existingAppNsMap, slice)
	if err != nil {
		return ctrl.Result{}, err, true
	}
	// Sweep the existing namespaces again to unbind any namespace that was not found in the configured list
	for existingAppNs := range existingAppNsMap {
		if !existingAppNsMap[existingAppNs].marked {
			err := r.unbindAppNamespace(ctx, slice, existingAppNs)
			if err != nil {
				log.Error(err, "Failed to unbind namespace from slice", "namespace", existingAppNs)
				return ctrl.Result{}, err, true
			}
			statusChanged = true
		}
	}
	if statusChanged {
		slice.Status.ApplicationNamespaces = labeledAppNsList
		err := r.Status().Update(ctx, slice)
		if err != nil {
			log.Error(err, "Failed to update slice status")
			return ctrl.Result{}, err, true
		}
		//TODO:
		//post changes to workersliceconfig
		sliceConfigName := slice.Name + "-" + controllers.ClusterName
		err = r.HubClient.UpdateAppNamespaces(ctx, sliceConfigName, labeledAppNsList)
		if err != nil {
			log.Error(err, "Failed to update workerslice status in controller cluster")
			return ctrl.Result{}, err, true
		}
		log.Info("updated onboarded namespaces to workersliceconfig")
	}
	return ctrl.Result{}, nil, false
}

func (r *SliceReconciler) reconcileAllowedNamespaces(ctx context.Context, slice *kubeslicev1beta1.Slice) error {
	log := logger.FromContext(ctx).WithValues("type", "allowedNamespaces")
	debugLog := log.V(1)
	//early exit if namespaceIsolation is empty
	if slice.Status.SliceConfig.NamespaceIsolationProfile == nil {
		return nil
	}
	//early exit if namespaceIsolation is not enabled
	if !slice.Status.SliceConfig.NamespaceIsolationProfile.IsolationEnabled {
		debugLog.Info("skipping reconcileAllowedNamespaces since isolation flag in not enabled")
		return nil
	}

	//cfgAllowedNsList contains list of allowedNamespaces from workersliceconfig
	var cfgAllowedNsList []string
	cfgAllowedNsList = append(cfgAllowedNsList, slice.Status.SliceConfig.NamespaceIsolationProfile.AllowedNamespaces...)
	for _, v := range allowedNamespacesByDefault {
		if !exists(cfgAllowedNsList, v) {
			cfgAllowedNsList = append(cfgAllowedNsList, v)
		}
	}
	debugLog.Info("reconciling", "allowedNamespaces", cfgAllowedNsList)

	// Get the list of existing namespaces that are tagged with the kube-slice label
	labeledNsList := &corev1.NamespaceList{}
	listOpts := []client.ListOption{
		client.HasLabels([]string{AllowedNamespaceSelectorLabelKey}),
	}
	err := r.List(ctx, labeledNsList, listOpts...)
	if err != nil {
		if !errors.IsNotFound(err) {
			log.Error(err, "Failed to list namespaces")
			return err
		}
	}

	// Sanitize the list of labeled namespaces to guard against someone manually editing the labels
	// attached to a namespace. The label key and value must match our syntax.
	// Also build a map for fast lookups in the rest of this function.
	labeledNsMap := make(map[string]string)
	for _, labeledNsObj := range labeledNsList.Items {
		labels := labeledNsObj.ObjectMeta.GetLabels()
		if labels[AllowedNamespaceSelectorLabelKey] != labeledNsObj.ObjectMeta.Name {
			log.Error(err, "Incorrect label", "Namespace", labeledNsObj.ObjectMeta.Name)
			labels[AllowedNamespaceSelectorLabelKey] = labeledNsObj.ObjectMeta.Name
			labeledNsObj.ObjectMeta.SetLabels(labels)
			err := r.Update(ctx, &labeledNsObj)
			if err != nil {
				log.Error(err, "Failed to update allowed ns label", "Namespace", labeledNsObj.ObjectMeta.Name)
				return err
			}
		}
		labeledNsMap[labeledNsObj.ObjectMeta.Name] = labels[AllowedNamespaceSelectorLabelKey]
	}

	// Label the namespace if needed
	for _, cfgAllowedNs := range cfgAllowedNsList {
		label, exists := labeledNsMap[cfgAllowedNs]
		if exists && label == cfgAllowedNs {
			continue
		}

		namespace := &corev1.Namespace{}
		err := r.Get(ctx, types.NamespacedName{Name: cfgAllowedNs}, namespace)
		if err != nil {
			log.Info("Failed to find namespace", "namespace", cfgAllowedNs)
			continue
		}

		// A namespace might not have any labels attached to it. Directly accessing the label map
		// leads to a crash for such namespaces.
		// If the label map is nil, create one and use the setter api to attach it to the namespace.
		nsLabels := namespace.ObjectMeta.GetLabels()
		if nsLabels == nil {
			nsLabels = make(map[string]string)
		}

		nsLabels[AllowedNamespaceSelectorLabelKey] = cfgAllowedNs
		namespace.ObjectMeta.SetLabels(nsLabels)
		err = r.Update(ctx, namespace)
		if err != nil {
			log.Error(err, "Failed to label namespace", "Namespace", cfgAllowedNs)
			return err
		}
		log.Info("Labeled namespace successfully", "namespace", cfgAllowedNs)
	}
	return nil
}

func (r *SliceReconciler) unbindAppNamespace(ctx context.Context, slice *kubeslicev1beta1.Slice, appNs string) error {
	log := logger.FromContext(ctx).WithValues("type", "appNamespaces")
	debuglog := log.V(1)
	namespace := &corev1.Namespace{}
	err := r.Get(ctx, types.NamespacedName{Name: appNs}, namespace)
	//namespace might be deleted by user/admin
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		log.Error(err, "NS unbind: Failed to find namespace", "namespace", appNs)
		return err
	}

	nsLabels := namespace.ObjectMeta.GetLabels()
	_, ok := nsLabels[controllers.ApplicationNamespaceSelectorLabelKey]
	if !ok {
		debuglog.Info("NS unbind: slice label not found", "namespace", appNs)
	} else {
		delete(nsLabels, controllers.ApplicationNamespaceSelectorLabelKey)
		namespace.ObjectMeta.SetLabels(nsLabels)
		err = r.Update(ctx, namespace)
		if err != nil {
			log.Error(err, "NS unbind: Failed to remove slice label", "namespace", appNs)
			return err
		}
	}
	// Delete network policy if present
	netPolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      slice.Name + "-" + appNs,
			Namespace: appNs,
		},
	}
	err = r.Delete(ctx, netPolicy)
	if err != nil && !errors.IsNotFound(err) {
		log.Error(err, "NS unbind: Failed to remove slice netpol", "namespace", appNs)
	}
	//remove the deployment annotations and labels from this namespace
	return r.deleteAnnotationsAndLabels(ctx, slice, appNs)
}

func (r *SliceReconciler) deleteAnnotationsAndLabels(ctx context.Context, slice *kubeslicev1beta1.Slice, appNs string) error {
	log := logger.FromContext(ctx).WithValues("type", "appNamespaces")
	deployList := appsv1.DeploymentList{}
	listOpts := []client.ListOption{
		client.InNamespace(appNs),
	}
	if err := r.List(ctx, &deployList, listOpts...); err != nil {
		log.Error(err, "Namespace offboarding:cannot list deployments under ns ", appNs)
		return err
	}
	for _, deploy := range deployList.Items {
		labels := deploy.Spec.Template.ObjectMeta.Labels
		if labels != nil {
			_, ok := labels[webhook.PodInjectLabelKey]
			if ok {
				delete(labels, webhook.PodInjectLabelKey)
			}
			sliceName, ok := labels[controllers.ApplicationNamespaceSelectorLabelKey]
			if ok && slice.Name == sliceName {
				delete(labels, controllers.ApplicationNamespaceSelectorLabelKey)
			}
		}
		podannotations := deploy.Spec.Template.ObjectMeta.Annotations
		if podannotations != nil {
			v, ok := podannotations["ns.networkservicemesh.io"]
			if ok && v == "vl3-service-"+slice.Name {
				delete(podannotations, "ns.networkservicemesh.io")
			}
		}
		deployannotations := deploy.ObjectMeta.GetAnnotations()
		if deployannotations != nil {
			_, ok := deployannotations["kubeslice.io/status"]
			if ok {
				delete(deployannotations, "kubeslice.io/status")
			}
		}
		if err := r.Update(ctx, &deploy); err != nil {
			log.Error(err, "Error deleting labels and annotations from deploy while namespace unbinding from slice", deploy.ObjectMeta.Name)
			return err
		}
		log.Info("Removed slice labels and annotations", "deployment", deploy.Name)
	}
	return nil
}

func (r *SliceReconciler) uninstallNetworkPolicies(ctx context.Context, slice *kubeslicev1beta1.Slice) error {
	log := r.Log.WithValues("type", "networkPolicy")
	for _, ns := range slice.Status.ApplicationNamespaces {
		// Delete network policy if present
		netPolicy := &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      slice.Name + "-" + ns,
				Namespace: ns,
			},
		}
		err := r.Delete(ctx, netPolicy)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			log.Error(err, "NS unbind: Failed to remove slice netpol", "namespace", ns)
		}
	}
	slice.Status.NetworkPoliciesInstalled = false
	return r.Status().Update(ctx, slice)
}

func (r *SliceReconciler) reconcileSliceNetworkPolicy(ctx context.Context, slice *kubeslicev1beta1.Slice) error {
	log := r.Log.WithValues("type", "networkPolicy")
	//early exit if namespaceIsolation is empty
	if slice.Status.SliceConfig.NamespaceIsolationProfile == nil {
		return nil
	}
	//Early Exit if Isolation is not enabled
	if !slice.Status.SliceConfig.NamespaceIsolationProfile.IsolationEnabled {
		// IsolationEnabled is either turned off or toggled off
		// if NetworkPoliciesInstalled is enabled, this means there are netpol installed in appnamespaces we need to remove
		if slice.Status.NetworkPoliciesInstalled {
			//IsolationEnabled toggled off by user/admin , uninstall nepol from app namespaces
			return r.uninstallNetworkPolicies(ctx, slice)
		}
		return nil
	}
	appNsList := &corev1.NamespaceList{}
	listOpts := []client.ListOption{
		client.MatchingLabels(map[string]string{controllers.ApplicationNamespaceSelectorLabelKey: slice.Name}),
	}
	err := r.List(ctx, appNsList, listOpts...)
	if err != nil {
		if !errors.IsNotFound(err) {
			log.Error(err, "Failed to list namespaces")
			return err
		}
	}
	for _, appNsObj := range appNsList.Items {
		err = r.installSliceNetworkPolicyInAppNs(ctx, slice, appNsObj.ObjectMeta.Name)
		if err != nil {
			log.Error(err, "Failed to install network policy", "namespace", appNsObj.ObjectMeta.Name)
			return err
		}
		log.Info("Installed netpol for namespace successfully", "namespace", appNsObj.ObjectMeta.Name)
	}
	slice.Status.NetworkPoliciesInstalled = true
	return r.Status().Update(ctx, slice)
}

func (r *SliceReconciler) installSliceNetworkPolicyInAppNs(ctx context.Context, slice *kubeslicev1beta1.Slice, appNs string) error {
	log := r.Log.WithValues("type", "networkPolicy")

	netPolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      slice.Name + "-" + appNs,
			Namespace: appNs,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{controllers.ApplicationNamespaceSelectorLabelKey: slice.Name},
							},
						},
					},
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{controllers.ApplicationNamespaceSelectorLabelKey: slice.Name},
							},
						},
					},
				},
			},
		},
	}

	var cfgAllowedNsList []string
	cfgAllowedNsList = slice.Status.SliceConfig.NamespaceIsolationProfile.AllowedNamespaces
	// traffic from "kubeslice-system","istio-system","kube-system" namespaces is allowed by default
	for _, v := range allowedNamespacesByDefault {
		if !exists(cfgAllowedNsList, v) {
			cfgAllowedNsList = append(cfgAllowedNsList, v)
		}
	}
	for _, allowedNs := range cfgAllowedNsList {
		ingressRule := networkingv1.NetworkPolicyPeer{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{AllowedNamespaceSelectorLabelKey: allowedNs},
			},
		}
		netPolicy.Spec.Ingress[0].From = append(netPolicy.Spec.Ingress[0].From, ingressRule)

		egressRule := networkingv1.NetworkPolicyPeer{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{AllowedNamespaceSelectorLabelKey: allowedNs},
			},
		}
		netPolicy.Spec.Egress[0].To = append(netPolicy.Spec.Egress[0].To, egressRule)
	}

	err := r.Update(ctx, netPolicy)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.Create(ctx, netPolicy)
		} else {
			return err
		}
	}
	log.Info("Updated network policy", "namespace", appNs)
	return nil
}

func (r *SliceReconciler) cleanupSliceNamespaces(ctx context.Context, slice *kubeslicev1beta1.Slice) {
	log := logger.FromContext(ctx).WithValues("type", "appNamespaces")

	// Get the list of existing namespaces that are tagged with the kubeslice label
	existingAppNsList := &corev1.NamespaceList{}
	listOpts := []client.ListOption{
		client.MatchingLabels(map[string]string{controllers.ApplicationNamespaceSelectorLabelKey: slice.Name}),
	}
	err := r.List(ctx, existingAppNsList, listOpts...)
	if err != nil {
		log.Error(err, "Failed to list namespaces")
	}

	for _, existingAppNsObj := range existingAppNsList.Items {
		err := r.unbindAppNamespace(ctx, slice, existingAppNsObj.ObjectMeta.Name)
		if err != nil {
			log.Error(err, "Failed to unbind namespace from slice", "namespace", existingAppNsObj.ObjectMeta.Name)
		}
	}
}

func exists(i []string, o string) bool {
	for _, v := range i {
		if v == o {
			return true
		}
	}
	return false
}

func buildAppNamespacesList(slice *kubeslicev1beta1.Slice) []string {
	var cfgAppNsList []string
	for _, qualifiedAppNs := range slice.Status.SliceConfig.NamespaceIsolationProfile.ApplicationNamespaces {
		// Ignore control plane namespace if it appears in the app namespace list
		if qualifiedAppNs == ControlPlaneNamespace {
			continue
		}
		cfgAppNsList = append(cfgAppNsList, qualifiedAppNs)
	}
	return cfgAppNsList
}
func (r *SliceReconciler) labelAppNamespaces(ctx context.Context, cfgAppNsList []string, existingAppNsMap map[string]*nsMarker, slice *kubeslicev1beta1.Slice) ([]string, bool, error) {
	labeledAppNsList := []string{}
	statusChanged := false
	log := logger.FromContext(ctx).WithValues("type", "appNamespaces")
	for _, cfgAppNs := range cfgAppNsList {
		if _, exists := existingAppNsMap[cfgAppNs]; exists {
			existingAppNsMap[cfgAppNs].marked = true
			labeledAppNsList = append(labeledAppNsList, cfgAppNs)
			continue
		}
		// label does not exists on namespace
		namespace := &corev1.Namespace{}
		err := r.Get(ctx, types.NamespacedName{Name: cfgAppNs}, namespace)
		if err != nil {
			log.Error(err, "Failed to find namespace", "namespace", cfgAppNs)
			continue
		}
		// A namespace might not have any labels attached to it. Directly accessing the label map
		// leads to a crash for such namespaces.
		// If the label map is nil, create one and use the setter api to attach it to the namespace.
		nsLabels := namespace.ObjectMeta.GetLabels()
		if nsLabels == nil {
			nsLabels = make(map[string]string)
		}
		nsLabels[controllers.ApplicationNamespaceSelectorLabelKey] = slice.Name
		namespace.ObjectMeta.SetLabels(nsLabels)

		err = r.Update(ctx, namespace)
		if err != nil {
			log.Error(err, "Failed to label namespace", "Namespace", cfgAppNs)
			return nil, false, err
		}
		log.Info("Labeled namespace successfully", "namespace", cfgAppNs)

		labeledAppNsList = append(labeledAppNsList, cfgAppNs)
		statusChanged = true
	}
	return labeledAppNsList, statusChanged, nil
}
