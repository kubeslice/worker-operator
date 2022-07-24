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
	"strings"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers"
	"github.com/kubeslice/worker-operator/pkg/logger"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
	//reconcile networkpolicy
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
	var cfgAppNsList []string
	for _, qualifiedAppNs := range slice.Status.SliceConfig.NamespaceIsolationProfile.ApplicationNamespaces {
		// Ignore control plane namespace if it appears in the app namespace list
		if qualifiedAppNs == ControlPlaneNamespace {
			continue
		}
		cfgAppNsList = append(cfgAppNsList, qualifiedAppNs)
	}
	debugLog.Info("reconciling", "applicationNamespaces", cfgAppNsList)

	// Get the list of existing namespaces that are tagged with the kubeslice label
	// existingAppNsList = list of all namespaces that have kubeslice label
	existingAppNsList := &corev1.NamespaceList{}
	listOpts := []client.ListOption{
		client.MatchingLabels(map[string]string{
			ApplicationNamespaceSelectorLabelKey: slice.Name,
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
	type nsMarker struct {
		ns     *corev1.Namespace
		marked bool
	}
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
	labeledAppNsList := []string{}
	statusChanged := false
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
		nsLabels[ApplicationNamespaceSelectorLabelKey] = slice.Name
		namespace.ObjectMeta.SetLabels(nsLabels)

		err = r.Update(ctx, namespace)
		if err != nil {
			log.Error(err, "Failed to label namespace", "Namespace", cfgAppNs)
			return ctrl.Result{}, err, true
		}
		log.Info("Labeled namespace successfully", "namespace", cfgAppNs)

		labeledAppNsList = append(labeledAppNsList, cfgAppNs)
		statusChanged = true
	}
	// Sweep the existing namespaces again to unbind any namespace that was not found in the configured list
	for existingAppNs, _ := range existingAppNsMap {
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
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			// fetch the latest slice
			if getErr := r.Get(ctx, types.NamespacedName{Name: slice.Name,Namespace: controllers.ControlPlaneNamespace}, slice); getErr != nil {
				return getErr
			}
			slice.Status.ApplicationNamespaces = labeledAppNsList
			err := r.Status().Update(ctx, slice)
			if err != nil {
				log.Error(err, "Failed to update Application Namespaces in slice status,retrying")
				return err
			}
			return nil
		})
		if err!=nil{
			return ctrl.Result{}, err, true
		}

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
	//early exit if NamespaceIsolationProfile is not defined or isolation is not enabled
	if slice.Status.SliceConfig.NamespaceIsolationProfile == nil || !slice.Status.SliceConfig.NamespaceIsolationProfile.IsolationEnabled {
		debugLog.Info("skipping reconcileAllowedNamespaces since isolation flag in not enabled")
		return nil
	}
	//cfgAllowedNsList contains list of allowedNamespaces from workersliceconfig
	var cfgAllowedNsList []string
	cfgAllowedNsList = append(cfgAllowedNsList, slice.Status.SliceConfig.NamespaceIsolationProfile.AllowedNamespaces...)
	// namespaces like "kubeslice-system","istio-system","kube-system" are always considered
	for _, v := range allowedNamespacesByDefault {
		if !exists(cfgAllowedNsList, v) {
			cfgAllowedNsList = append(cfgAllowedNsList, v)
		}
	}
	log.Info("reconciling", "allowedNamespaces", cfgAllowedNsList)

	// Get the list of existing namespaces that are tagged with the kube-slice label for allowed NS
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
	// Convert the list into a map for faster lookups. Will come in handy when we compare
	// existing namespaces against configured namespaces.
	type nsMarker struct {
		ns     corev1.Namespace
		marked bool
	}
	existingAllowedNsMap := make(map[string]*nsMarker)

	for _, existingAllowedNsObj := range labeledNsList.Items {
		existingAllowedNsMap[existingAllowedNsObj.ObjectMeta.Name] = &nsMarker{
			ns:     existingAllowedNsObj,
			marked: false,
		}
	}
	// Compare the existing list with the configured list.
	// If a namespace is found in the existing list, mark it to indicate that it has been verified
	// to be valid as it is present in the configured list as well.
	labeledAllowedNsList := []string{}
	statusChanged := false
	// annotationApplied will be made true if any new slice Name is appended to a namespace annotation. In that case, we would want to update the status.AllowedNamespaces
	var annotationApplied bool
	for _, cfgAllowedNs := range cfgAllowedNsList {
		if v, exists := existingAllowedNsMap[cfgAllowedNs]; exists {
			existingAllowedNsMap[cfgAllowedNs].marked = true
			if annotationApplied, err = r.annotateAllowedNamespace(ctx, slice, &v.ns); err != nil {
				log.Error(err, "Error annotating allowedNamespace", "Namespace", v.ns.Name)
			}
			labeledAllowedNsList = append(labeledAllowedNsList, cfgAllowedNs)
			if annotationApplied {
				statusChanged = true
			}
			continue
		}
		// label does not exists on namespace
		namespace := &corev1.Namespace{}
		err := r.Get(ctx, types.NamespacedName{Name: cfgAllowedNs}, namespace)
		if err != nil {
			log.Error(err, "Failed to find namespace", "namespace", cfgAllowedNs)
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

		labeledAllowedNsList = append(labeledAllowedNsList, cfgAllowedNs)
		statusChanged = true
	}
	// Sweep the existing namespaces again to unbind any namespace that was not found in the configured list
	// Sweep the existing namespaces again to unbind any namespace that was not found in the configured list
	for existingAllowedNs, _ := range existingAllowedNsMap {
		if !existingAllowedNsMap[existingAllowedNs].marked {
			err := r.unbindAllowedNamespace(ctx, existingAllowedNs, slice.Name)
			if err != nil {
				log.Error(err, "Failed to unbind namespace from slice", "namespace", existingAllowedNs)
				return err
			}
			log.Info("unbind allowed namespace success", "namespace", existingAllowedNs)
			statusChanged = true
		}
	}
	if statusChanged {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			// fetch the latest slice
			if getErr := r.Get(ctx, types.NamespacedName{Name: slice.Name,Namespace: controllers.ControlPlaneNamespace}, slice); getErr != nil {
				return getErr
			}
			slice.Status.AllowedNamespaces = labeledAllowedNsList
			err := r.Status().Update(ctx, slice)
			if err != nil {
				log.Error(err, "Failed to update Allowed Namespaces in slice status,retrying")
				return err
			}
			return nil
		})
		if err!=nil{
			return err
		}
	}
	return nil
}

func (r *SliceReconciler) annotateAllowedNamespace(ctx context.Context, slice *kubeslicev1beta1.Slice, allowedNamespace *corev1.Namespace) (bool, error) {
	annotations := allowedNamespace.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	} 
	v, ok := annotations[AllowedNamespaceAnnotationKey]
	if !ok {
		// AllowedNamespaceAnnotationKey not present
		annotations[AllowedNamespaceAnnotationKey] = slice.Name
		allowedNamespace.ObjectMeta.Annotations = annotations
		return true, r.Update(ctx, allowedNamespace)
	}
	// AllowedNamespaceAnnotationKey present, append the comma seperated sliceName to value
	// eg : kubeslice.io/trafficAllowedToSlices: "slice-1,slice-2,slice-3"
	// create an array of slices and check if the slice name already exists
	a := strings.Split(v, ",")
	if !exists(a, slice.Name) {
		annotations[AllowedNamespaceAnnotationKey] = v + "," + slice.Name
		allowedNamespace.ObjectMeta.Annotations = annotations
		return true, r.Update(ctx, allowedNamespace)
	}
	return false, nil
}

// unbindAllowedNamespace will remove the slice name from annotation
// in case it the last slice, it will also remove kubeslice Allowed Namespace label
func (r *SliceReconciler) unbindAllowedNamespace(ctx context.Context, allowedNs, sliceName string) error {
	log := logger.FromContext(ctx).WithValues("type", "allowedNamespace")
	namespace := &corev1.Namespace{}
	err := r.Get(ctx, types.NamespacedName{Name: allowedNs}, namespace)
	if err != nil {
		log.Error(err, "NS unbind: Failed to find namespace", "namespace", allowedNs)
		return err
	}

	//early exit if annotations are nil or AllowedNamespaceAnnotationKey is not present
	annotations := namespace.GetAnnotations()
	if annotations == nil {
		return nil
	}
	v,present := annotations[AllowedNamespaceAnnotationKey]
	if !present{
		return nil
	}
	a := strings.Split(v, ",")
	if exists(a, sliceName) {
		if len(a) == 1 {
			// last slice to offboard
			delete(annotations, AllowedNamespaceAnnotationKey)
			namespace.SetAnnotations(annotations)
			//remove kubeslice allowed namespace label
			labels := namespace.GetLabels()
			_, ok := labels[AllowedNamespaceSelectorLabelKey]
			if ok {
				delete(labels, AllowedNamespaceSelectorLabelKey)
				namespace.SetLabels(labels)
			}
			return r.Update(ctx, namespace)
		} else {
			//remove the sliceName from annotation
			toDeleteSlice := indexOf(a, sliceName)
			if toDeleteSlice == -1 {
				return nil
			}
			a = append(a[:toDeleteSlice], a[toDeleteSlice+1:]...)
			annotations[AllowedNamespaceAnnotationKey] = strings.Join(a, ",")
			namespace.SetAnnotations(annotations)
			return r.Update(ctx, namespace)
		}
	}
	return nil
}

func indexOf(a []string, b string) int {
	for i, j := range a {
		if j == b {
			return i
		}
	}
	return -1
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
	_, ok := nsLabels[ApplicationNamespaceSelectorLabelKey]
	if !ok {
		debuglog.Info("NS unbind: slice label not found", "namespace", appNs)
	} else {
		delete(nsLabels, ApplicationNamespaceSelectorLabelKey)
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
			_, ok := labels["kubeslice.io/pod-type"]
			if ok {
				delete(labels, "kubeslice.io/pod-type")
			}
			sliceName, ok := labels["kubeslice.io/slice"]
			if ok && slice.Name == sliceName {
				delete(labels, "kubeslice.io/slice")
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
		client.MatchingLabels(map[string]string{ApplicationNamespaceSelectorLabelKey: slice.Name}),
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
				networkingv1.NetworkPolicyIngressRule{
					From: []networkingv1.NetworkPolicyPeer{
						networkingv1.NetworkPolicyPeer{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{ApplicationNamespaceSelectorLabelKey: slice.Name},
							},
						},
					},
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				networkingv1.NetworkPolicyEgressRule{
					To: []networkingv1.NetworkPolicyPeer{
						networkingv1.NetworkPolicyPeer{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{ApplicationNamespaceSelectorLabelKey: slice.Name},
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
		client.MatchingLabels(map[string]string{ApplicationNamespaceSelectorLabelKey: slice.Name}),
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
	// unbind allowed Namespaces
	for _, namespace := range slice.Status.AllowedNamespaces {
		if err := r.unbindAllowedNamespace(ctx, namespace, slice.Name); err != nil {
			log.Error(err, "failed to unbind allowedNamespace", "namespace", namespace)
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
