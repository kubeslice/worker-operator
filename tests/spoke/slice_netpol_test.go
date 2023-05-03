package spoke_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers"
	slicepkg "github.com/kubeslice/worker-operator/controllers/slice"
	"github.com/kubeslice/worker-operator/tests/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	timeout  = time.Second * 60
	interval = time.Millisecond * 250
)

var _ = Describe("SliceNetpol", func() {
	var slice *kubeslicev1beta1.Slice
	var svc *corev1.Service
	var createdSlice *kubeslicev1beta1.Slice
	var appNs *corev1.Namespace
	var allowedNs *corev1.Namespace
	Context("With slice CR created and application namespaces specified ", func() {
		BeforeEach(func() {
			// Prepare k8s objects for slice and kubeslice-dns service
			slice = &kubeslicev1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice-netpol",
					Namespace: "kubeslice-system",
				},
				Spec: kubeslicev1beta1.SliceSpec{},
			}

			svc = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kubeslice-dns",
					Namespace: "kubeslice-system",
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: "10.0.0.20",
					Ports: []corev1.ServicePort{{
						Port: 52,
					}},
				},
			}
			appNs = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "iperf",
				},
			}
			allowedNs = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kube-allowed",
				},
			}
			createdSlice = &kubeslicev1beta1.Slice{}
			// Cleanup after each test
			DeferCleanup(func() {
				ctx := context.Background()
				Expect(k8sClient.Delete(ctx, svc)).Should(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, svc)
					return errors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue())
				Expect(k8sClient.Delete(ctx, slice)).Should(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: slice.Name, Namespace: slice.Namespace}, slice)
					return errors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue())
			})
		})
		It("Should reconcile application namespaces", func() {
			Expect(k8sClient.Create(ctx, svc)).Should(Succeed())
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, appNs)).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-slice-netpol",
					Namespace: "kubeslice-system",
				}, createdSlice)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-slice-netpol",
					Namespace: "kubeslice-system",
				}, createdSlice)
				if err != nil {
					return err
				}
				createdSlice.Status.SliceConfig = &kubeslicev1beta1.SliceConfig{
					NamespaceIsolationProfile: &kubeslicev1beta1.NamespaceIsolationProfile{
						IsolationEnabled: true,
						ApplicationNamespaces: []string{
							"iperf",
						},
						AllowedNamespaces: []string{},
					},
				}
				err = k8sClient.Status().Update(ctx, createdSlice)
				return err
			})
			Expect(err).To(BeNil())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "iperf"}, appNs)
				if err != nil {
					return false
				}
				labels := appNs.ObjectMeta.GetLabels()
				if labels == nil {
					return false
				}
				sliceLabel, ok := labels[controllers.ApplicationNamespaceSelectorLabelKey]
				if !ok {
					return false
				}
				Expect(sliceLabel).To(Equal(slice.Name))
				return true
			}, timeout, interval).Should(BeTrue())

			//delete the labels and re-test it
			labels := appNs.ObjectMeta.GetLabels()
			delete(labels, controllers.ApplicationNamespaceSelectorLabelKey)
			appNs.ObjectMeta.SetLabels(labels)
			Expect(k8sClient.Update(ctx, appNs)).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "iperf"}, appNs)
				if err != nil {
					return false
				}
				labels := appNs.ObjectMeta.GetLabels()
				if labels == nil {
					return false
				}
				sliceLabel, ok := labels[controllers.ApplicationNamespaceSelectorLabelKey]
				if !ok {
					return false
				}
				Expect(sliceLabel).To(Equal(slice.Name))
				return true
			}, timeout, interval).Should(BeTrue())
		})

		It("Should reconcile allowed namespaces", func() {
			Expect(k8sClient.Create(ctx, svc)).Should(Succeed())
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, allowedNs)).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-slice-netpol",
					Namespace: "kubeslice-system",
				}, createdSlice)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-slice-netpol",
					Namespace: "kubeslice-system",
				}, createdSlice)
				if err != nil {
					return err
				}
				createdSlice.Status.SliceConfig = &kubeslicev1beta1.SliceConfig{
					NamespaceIsolationProfile: &kubeslicev1beta1.NamespaceIsolationProfile{
						IsolationEnabled:      true,
						ApplicationNamespaces: []string{},
						AllowedNamespaces: []string{
							"kube-allowed",
						},
					},
				}
				err = k8sClient.Status().Update(ctx, createdSlice)
				return err
			})
			Expect(err).To(BeNil())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "kube-allowed"}, appNs)
				if err != nil {
					return false
				}
				labels := appNs.ObjectMeta.GetLabels()
				if labels == nil {
					return false
				}
				sliceLabel, ok := labels[slicepkg.AllowedNamespaceSelectorLabelKey]
				if !ok {
					return false
				}
				Expect(sliceLabel).To(Equal(allowedNs.Name))

				annotation := appNs.ObjectMeta.GetAnnotations()
				sliceAnnotation, ok := annotation[slicepkg.AllowedNamespaceAnnotationKey]
				if !ok {
					return false
				}
				Expect(sliceAnnotation).To(Equal(slice.Name))
				return true
			}, timeout, interval).Should(BeTrue())
		})
		It("Should annotate allowed namespace with slice annotation", func() {
			Expect(k8sClient.Create(ctx, svc)).Should(Succeed())
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			slice2 := &kubeslicev1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "demoslice-nw",
					Namespace: "kubeslice-system",
				},
				Spec: kubeslicev1beta1.SliceSpec{},
			}
			Expect(k8sClient.Create(ctx, slice2)).Should(Succeed())

			//update slice1
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-slice-netpol",
					Namespace: "kubeslice-system",
				}, createdSlice)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-slice-netpol",
					Namespace: "kubeslice-system",
				}, createdSlice)
				if err != nil {
					return err
				}
				createdSlice.Status.SliceConfig = &kubeslicev1beta1.SliceConfig{
					NamespaceIsolationProfile: &kubeslicev1beta1.NamespaceIsolationProfile{
						IsolationEnabled:      true,
						ApplicationNamespaces: []string{},
						AllowedNamespaces: []string{
							"kube-allowed",
						},
					},
				}
				err = k8sClient.Status().Update(ctx, createdSlice)
				return err
			})
			Expect(err).To(BeNil())

			//update slice2
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "demoslice-nw",
					Namespace: "kubeslice-system",
				}, createdSlice)
				if err != nil {
					return err
				}
				createdSlice.Status.SliceConfig = &kubeslicev1beta1.SliceConfig{
					NamespaceIsolationProfile: &kubeslicev1beta1.NamespaceIsolationProfile{
						IsolationEnabled:      true,
						ApplicationNamespaces: []string{},
						AllowedNamespaces: []string{
							"kube-allowed",
						},
					},
				}
				err = k8sClient.Status().Update(ctx, createdSlice)
				return err
			})
			Expect(err).To(BeNil())

			//verify if annotation is added on allowedNs
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "kube-allowed"}, appNs)
				if err != nil {
					return false
				}
				labels := appNs.ObjectMeta.GetLabels()
				if labels == nil {
					return false
				}
				sliceLabel, ok := labels[slicepkg.AllowedNamespaceSelectorLabelKey]
				if !ok {
					return false
				}
				Expect(sliceLabel).To(Equal(allowedNs.Name))

				annotation := appNs.ObjectMeta.GetAnnotations()
				sliceAnnotation, ok := annotation[slicepkg.AllowedNamespaceAnnotationKey]
				if !ok {
					return false
				}
				a := strings.Split(sliceAnnotation, ",")
				if len(a) != 2 {
					return false
				}
				if !exists(a, "demoslice-nw") && !exists(a, "test-slice-netpol") {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			//delete slice2 and verify annotation is removed
			Expect(k8sClient.Delete(ctx, slice2)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "kube-allowed"}, appNs)
				if err != nil {
					return false
				}
				labels := appNs.ObjectMeta.GetLabels()
				if labels == nil {
					return false
				}
				sliceLabel, ok := labels[slicepkg.AllowedNamespaceSelectorLabelKey]
				if !ok {
					return false
				}
				Expect(sliceLabel).To(Equal(allowedNs.Name))

				annotation := appNs.ObjectMeta.GetAnnotations()
				sliceAnnotation, ok := annotation[slicepkg.AllowedNamespaceAnnotationKey]
				if !ok {
					return false
				}
				return sliceAnnotation == slice.Name
			}, timeout, interval).Should(BeTrue())

			//finally delete the slice and verify it kubeslice-allowedNS label is removed
			Expect(k8sClient.Delete(ctx, slice)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: slice.Name, Namespace: slice.Namespace}, slice)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "kube-allowed"}, appNs)
				if err != nil {
					return false
				}
				labels := appNs.ObjectMeta.GetLabels()

				//expect kubeslice label to be gone
				if labels == nil {
					return true
				}
				_, ok := labels[slicepkg.AllowedNamespaceSelectorLabelKey]
				if !ok {
					return true
				}

				annotation := appNs.ObjectMeta.GetAnnotations()
				_, ok = annotation[slicepkg.AllowedNamespaceAnnotationKey]
				return !ok
			}, timeout, interval).Should(BeTrue())

			slice = &kubeslicev1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice-netpol",
					Namespace: "kubeslice-system",
				},
				Spec: kubeslicev1beta1.SliceSpec{},
			}
			//re-create it back for defer cleanup to pass
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
		})
		It("Should install network policy on application namespaces", func() {
			Expect(k8sClient.Create(ctx, svc)).Should(Succeed())
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			iperfNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "iperf-demo",
				},
			}
			Expect(k8sClient.Create(ctx, iperfNs)).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-slice-netpol",
					Namespace: "kubeslice-system",
				}, createdSlice)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-slice-netpol",
					Namespace: "kubeslice-system",
				}, createdSlice)
				if err != nil {
					return err
				}
				createdSlice.Status.SliceConfig = &kubeslicev1beta1.SliceConfig{
					NamespaceIsolationProfile: &kubeslicev1beta1.NamespaceIsolationProfile{
						IsolationEnabled: true,
						ApplicationNamespaces: []string{
							"iperf-demo",
						},
						AllowedNamespaces: []string{},
					},
				}
				err = k8sClient.Status().Update(ctx, createdSlice)
				return err
			})
			Expect(err).To(BeNil())
			//verify if the namespace is labelled correctly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "iperf-demo"}, appNs)
				if err != nil {
					return false
				}
				labels := appNs.ObjectMeta.GetLabels()
				if labels == nil {
					return false
				}
				sliceLabel, ok := labels[controllers.ApplicationNamespaceSelectorLabelKey]
				if !ok {
					return false
				}
				Expect(sliceLabel).To(Equal(slice.Name))
				return true
			}, timeout, interval).Should(BeTrue())

			netpol := networkingv1.NetworkPolicy{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: slice.Name + "-" + iperfNs.Name, Namespace: iperfNs.Name}, &netpol)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			policyTypes := netpol.Spec.PolicyTypes
			Expect(len(policyTypes)).To(Equal(len([]string{string(networkingv1.PolicyTypeIngress), string(networkingv1.PolicyTypeEgress)})))
			Expect(policyTypes[0]).To(Equal(networkingv1.PolicyTypeIngress))
			Expect(policyTypes[1]).To(Equal(networkingv1.PolicyTypeEgress))
			//test ingress rules
			ingressRules := netpol.Spec.Ingress
			ingressRule := ingressRules[0]
			nsSelectorLabel := ingressRule.From[0].NamespaceSelector.MatchLabels
			Expect(nsSelectorLabel[controllers.ApplicationNamespaceSelectorLabelKey]).To(Equal(slice.Name))
			//test egress rules
			egressRules := netpol.Spec.Egress
			egressRule := egressRules[0]
			nsSelectorLabel = egressRule.To[0].NamespaceSelector.MatchLabels
			Expect(nsSelectorLabel[controllers.ApplicationNamespaceSelectorLabelKey]).To(Equal(slice.Name))
		})

		It("Should update network policy to include allowed namespaces", func() {
			Expect(k8sClient.Create(ctx, svc)).Should(Succeed())
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			applicationNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "application-ns",
				},
			}
			allowedNamespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "allowed-ns",
				},
			}
			Expect(k8sClient.Create(ctx, applicationNs)).Should(Succeed())
			Expect(k8sClient.Create(ctx, allowedNamespace)).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-slice-netpol",
					Namespace: "kubeslice-system",
				}, createdSlice)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-slice-netpol",
					Namespace: "kubeslice-system",
				}, createdSlice)
				if err != nil {
					return err
				}
				createdSlice.Status.SliceConfig = &kubeslicev1beta1.SliceConfig{
					NamespaceIsolationProfile: &kubeslicev1beta1.NamespaceIsolationProfile{
						IsolationEnabled: true,
						ApplicationNamespaces: []string{
							"application-ns",
						},
						AllowedNamespaces: []string{
							"allowed-ns",
						},
					},
				}
				err = k8sClient.Status().Update(ctx, createdSlice)
				return err
			})
			Expect(err).To(BeNil())

			//verify if the app namespace is labelled correctly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "application-ns"}, appNs)
				if err != nil {
					return false
				}
				labels := appNs.ObjectMeta.GetLabels()
				if labels == nil {
					return false
				}
				sliceLabel, ok := labels[controllers.ApplicationNamespaceSelectorLabelKey]
				if !ok {
					return false
				}
				Expect(sliceLabel).To(Equal(slice.Name))
				return true
			}, timeout, interval).Should(BeTrue())

			//verify if the allowed namespace is labelled correctly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "allowed-ns"}, appNs)
				if err != nil {
					return false
				}
				labels := appNs.ObjectMeta.GetLabels()
				if labels == nil {
					return false
				}
				sliceLabel, ok := labels[slicepkg.AllowedNamespaceSelectorLabelKey]
				if !ok {
					return false
				}
				Expect(sliceLabel).To(Equal(allowedNamespace.Name))
				return true
			}, timeout, interval).Should(BeTrue())

			//fetch network policy
			netpol := networkingv1.NetworkPolicy{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: slice.Name + "-" + applicationNs.Name, Namespace: applicationNs.Name}, &netpol)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			//test ingress rules
			ingressRules := netpol.Spec.Ingress
			ingressRule := ingressRules[0]
			//applicationNamespace ingress rule
			nsSelectorLabelApp := ingressRule.From[0].NamespaceSelector.MatchLabels
			Expect(nsSelectorLabelApp[controllers.ApplicationNamespaceSelectorLabelKey]).To(Equal(slice.Name))
			//allowedNamespace ingress rule
			nsSelectorLabelAllowed := ingressRule.From[1].NamespaceSelector.MatchLabels
			Expect(nsSelectorLabelAllowed[slicepkg.AllowedNamespaceSelectorLabelKey]).To(Equal(allowedNamespace.Name))

			//test egress rules
			egressRules := netpol.Spec.Egress
			egressRule := egressRules[0]
			//applicationNamespace egress rule
			nsSelectorLabelApp = egressRule.To[0].NamespaceSelector.MatchLabels
			Expect(nsSelectorLabelApp[controllers.ApplicationNamespaceSelectorLabelKey]).To(Equal(slice.Name))
			//allowedNamespace egress rule
			nsSelectorLabelAllowed = egressRule.To[1].NamespaceSelector.MatchLabels
			Expect(nsSelectorLabelAllowed[slicepkg.AllowedNamespaceSelectorLabelKey]).To(Equal(allowedNamespace.Name))
		})

		It("Should raise an event in case of networkpolicy violation", func() {
			Expect(k8sClient.Create(ctx, svc)).Should(Succeed())
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			applicationNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "application-ns-netpol",
				},
			}
			allowedNamespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "allowed-ns-netpol",
				},
			}
			Expect(k8sClient.Create(ctx, applicationNs)).Should(Succeed())
			Expect(k8sClient.Create(ctx, allowedNamespace)).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-slice-netpol",
					Namespace: "kubeslice-system",
				}, createdSlice)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-slice-netpol",
					Namespace: "kubeslice-system",
				}, createdSlice)
				if err != nil {
					return err
				}
				createdSlice.Status.SliceConfig = &kubeslicev1beta1.SliceConfig{
					NamespaceIsolationProfile: &kubeslicev1beta1.NamespaceIsolationProfile{
						IsolationEnabled: true,
						ApplicationNamespaces: []string{
							"application-ns-netpol",
						},
						AllowedNamespaces: []string{
							"allowed-ns-netpol",
						},
					},
				}
				err = k8sClient.Status().Update(ctx, createdSlice)
				return err
			})
			Expect(err).To(BeNil())
			//verify if the app namespace is labelled correctly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "application-ns-netpol"}, appNs)
				if err != nil {
					return false
				}
				labels := appNs.ObjectMeta.GetLabels()
				if labels == nil {
					return false
				}
				sliceLabel, ok := labels[controllers.ApplicationNamespaceSelectorLabelKey]
				if !ok {
					return false
				}
				Expect(sliceLabel).To(Equal(slice.Name))
				return true
			}, timeout, interval).Should(BeTrue())

			//verify if the allowed namespace is labelled correctly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "allowed-ns-netpol"}, appNs)
				if err != nil {
					return false
				}
				labels := appNs.ObjectMeta.GetLabels()
				if labels == nil {
					return false
				}
				sliceLabel, ok := labels[slicepkg.AllowedNamespaceSelectorLabelKey]
				if !ok {
					return false
				}
				Expect(sliceLabel).To(Equal(allowedNamespace.Name))
				return true
			}, timeout, interval).Should(BeTrue())

			//fetch network policy
			netpol := networkingv1.NetworkPolicy{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: slice.Name + "-" + applicationNs.Name, Namespace: applicationNs.Name}, &netpol)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			//install networkpolicy that violates the existing netpol and expect the event to be created on slice
			Expect(k8sClient.Create(ctx, getNetpol())).Should(Succeed())
			eventList := corev1.EventList{}
			//wait till netpol_controller creates an event
			Eventually(func() bool {
				opts := []client.ListOption{
					client.InNamespace(slice.Namespace),
				}
				err = k8sClient.List(ctx, &eventList, opts...)
				return len(eventList.Items) > 1
			}, timeout, interval).Should(BeTrue())
			Expect(eventList.Items[1].InvolvedObject.Kind).Should(Equal("Slice"))
			Expect(eventList.Items[1].Reason).Should(Equal("Scope widened with reason - IPBlock violation"))

			m, err := utils.GetGaugeMetricFromRegistry(MetricRegistry, "kubeslice_netpol_violations_active", map[string]string{
				"slice":                         "test-slice-netpol",
				"slice_cluster":                 "cluster-test",
				"slice_namespace":               "application-ns-netpol",
				"slice_networkpolicy":           "netpol-violation",
				"slice_networkpolicy_violation": "ipblock",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(m).To(Equal(1.0))
		})
		It("Should uninstall netpol when isolationEnabled is toggled off", func() {
			Expect(k8sClient.Create(ctx, svc)).Should(Succeed())
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			iperfNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "iperf-demo-foo",
				},
			}
			Expect(k8sClient.Create(ctx, iperfNs)).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-slice-netpol",
					Namespace: "kubeslice-system",
				}, createdSlice)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-slice-netpol",
					Namespace: "kubeslice-system",
				}, createdSlice)
				if err != nil {
					return err
				}
				createdSlice.Status.SliceConfig = &kubeslicev1beta1.SliceConfig{
					NamespaceIsolationProfile: &kubeslicev1beta1.NamespaceIsolationProfile{
						IsolationEnabled: true,
						ApplicationNamespaces: []string{
							"iperf-demo-foo",
						},
						AllowedNamespaces: []string{},
					},
				}
				err = k8sClient.Status().Update(ctx, createdSlice)
				return err
			})
			Expect(err).To(BeNil())
			//verify if the namespace is labelled correctly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "iperf-demo-foo"}, appNs)
				if err != nil {
					return false
				}
				labels := appNs.ObjectMeta.GetLabels()
				if labels == nil {
					return false
				}
				sliceLabel, ok := labels[controllers.ApplicationNamespaceSelectorLabelKey]
				if !ok {
					return false
				}
				Expect(sliceLabel).To(Equal(slice.Name))
				return true
			}, timeout, interval).Should(BeTrue())

			netpol := networkingv1.NetworkPolicy{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: slice.Name + "-" + iperfNs.Name, Namespace: iperfNs.Name}, &netpol)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			//toggle the IsolationEnabled to false
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-slice-netpol",
					Namespace: "kubeslice-system",
				}, createdSlice)
				if err != nil {
					return err
				}
				createdSlice.Status.SliceConfig.NamespaceIsolationProfile.IsolationEnabled = false
				err = k8sClient.Status().Update(ctx, createdSlice)
				return err
			})
			Expect(err).To(BeNil())
			netpol = networkingv1.NetworkPolicy{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: slice.Name + "-" + iperfNs.Name, Namespace: iperfNs.Name}, &netpol)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

		})
		It("should remove labels and annotations from application deploy,once ns is offboarded", func() {
			Expect(k8sClient.Create(ctx, svc)).Should(Succeed())
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			applicationNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "application-iperf-ns",
				},
			}
			allowedNamespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "allowed-ns-01",
				},
			}
			Expect(k8sClient.Create(ctx, applicationNs)).Should(Succeed())
			Expect(k8sClient.Create(ctx, allowedNamespace)).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-slice-netpol",
					Namespace: "kubeslice-system",
				}, createdSlice)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-slice-netpol",
					Namespace: "kubeslice-system",
				}, createdSlice)
				if err != nil {
					return err
				}
				createdSlice.Status.SliceConfig = &kubeslicev1beta1.SliceConfig{
					NamespaceIsolationProfile: &kubeslicev1beta1.NamespaceIsolationProfile{
						IsolationEnabled: true,
						ApplicationNamespaces: []string{
							"application-iperf-ns",
						},
						AllowedNamespaces: []string{
							"allowed-ns-01",
						},
					},
				}
				err = k8sClient.Status().Update(ctx, createdSlice)
				return err
			})
			Expect(err).To(BeNil())
			//verify if the namespace is onboarded correctly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "application-iperf-ns"}, appNs)
				if err != nil {
					return false
				}
				labels := appNs.ObjectMeta.GetLabels()
				if labels == nil {
					return false
				}
				sliceLabel, ok := labels[controllers.ApplicationNamespaceSelectorLabelKey]
				if !ok {
					return false
				}
				Expect(sliceLabel).To(Equal(slice.Name))
				return true
			}, timeout, interval).Should(BeTrue())
			//onbaord deploy in iperf namespace
			Expect(k8sClient.Create(ctx, getPod())).Should(Succeed())
			// update slice status to ofboard namespace
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-slice-netpol",
					Namespace: "kubeslice-system",
				}, createdSlice)
				if err != nil {
					return err
				}
				createdSlice.Status.SliceConfig.NamespaceIsolationProfile.ApplicationNamespaces = nil
				err = k8sClient.Status().Update(ctx, createdSlice)
				return err
			})
			Expect(err).To(BeNil())
			//get the pod and verify if labels and annotations are removed
			Eventually(func() bool {
				createdPod := corev1.Pod{}
				podKey := types.NamespacedName{Name: "iperf-sleep", Namespace: "application-iperf-ns"}
				err := k8sClient.Get(ctx, podKey, &createdPod)
				if err != nil {
					return false
				}
				labels := createdPod.ObjectMeta.Labels
				fmt.Println(labels)
				_, ok := labels["kubeslice.io/pod-type"]
				if ok {
					return false
				}
				_, ok = labels["kubeslice.io/slice"]
				if ok {
					return false
				}
				_, ok = createdPod.ObjectMeta.Annotations["ns.networkservicemesh.io"]
				if ok {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})
	})
})

func getNetpol() *networkingv1.NetworkPolicy {
	netPolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "netpol-violation",
			Namespace: "application-ns-netpol",
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				networkingv1.NetworkPolicyIngressRule{
					From: []networkingv1.NetworkPolicyPeer{
						networkingv1.NetworkPolicyPeer{
							IPBlock: &networkingv1.IPBlock{
								CIDR: "172.17.0.0/16",
							},
						},
					},
				},
			},
		},
	}
	return netPolicy
}

func getPod() *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "iperf-sleep",
			Namespace: "application-iperf-ns",
			Labels: map[string]string{
				"app": "iperf-sleep",
			},
			Annotations: map[string]string{
				"kubeslice.io/status": "injected",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "iperf",
					Image:           "mlabbe/iperf",
					ImagePullPolicy: "Always",
					Command:         []string{"/bin/sleep", "3650d"},
				},
			},
		},
	}
	return pod
}

func getDeploy() *appsv1.Deployment {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "iperf-sleep",
			Namespace: "application-iperf-ns",
			Labels: map[string]string{
				"app": "iperf-sleep",
			},
			Annotations: map[string]string{
				"kubeslice.io/status": "injected",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "iperf-sleep",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                   "iperf-sleep",
						"kubeslice.io/pod-type": "app",
						"kubeslice.io/slice":    "test-slice-netpol",
					},
					Annotations: map[string]string{
						"ns.networkservicemesh.io": "vl3-service-test-slice-netpol",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "iperf",
							Image:           "mlabbe/iperf",
							ImagePullPolicy: "Always",
							Command:         []string{"/bin/sleep", "3650d"},
						},
					},
				},
			},
		},
	}
	return deploy
}

func exists(i []string, o string) bool {
	for _, v := range i {
		if v == o {
			return true
		}
	}
	return false
}
