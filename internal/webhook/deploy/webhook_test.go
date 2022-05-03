package deploy_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kubeslice/operator/internal/webhook/deploy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Deploy Webhook", func() {

	Describe("MutationRequired", func() {
		Context("New deployment without proper annotation", func() {

			table := []metav1.ObjectMeta{
				{}, // with empty meta
				{
					Annotations: map[string]string{}, // with empty annotations
				},
				{
					Annotations: map[string]string{
						"kubeslice.io/status": "",
					}, // with empty value for status key
				},
				{
					Annotations: map[string]string{
						"kubeslice.io/slice": "",
					}, // with empty value for slice key
				},
				{
					Annotations: map[string]string{
						"kubeslice.io/status": "not injected",
					}, // with different value for status key
				},
			}

			It("should not enable injection", func() {

				for _, meta := range table {
					is := deploy.MutationRequired(meta)
					Expect(is).To(BeFalse())
				}

			})
		})

		Context("New deployment with proper annotation", func() {

			table := []metav1.ObjectMeta{
				{
					Annotations: map[string]string{
						"kubeslice.io/slice": "green",
					}, // with proper annotations
				},
				{
					Annotations: map[string]string{
						"kubeslice.io/slice":  "green",
						"kubeslice.io/status": "",
					}, // with empty value for status key
				},
				{
					Annotations: map[string]string{
						"kubeslice.io/slice":  "green",
						"kubeslice.io/status": "not injected",
					}, // with different value for status key
				},
			}

			It("should enable injection", func() {

				for _, meta := range table {
					is := deploy.MutationRequired(meta)
					Expect(is).To(BeTrue())
				}

			})
		})

		Context("Already Injected", func() {

			table := []metav1.ObjectMeta{
				{
					Annotations: map[string]string{
						"kubeslice.io/status": "injected",
					}, // with injection status
				},
				{
					Labels: map[string]string{
						"kubeslice.io/pod-type": "app",
					}, // with pod type set
				},
				{
					Annotations: map[string]string{
						"kubeslice.io/status": "injected",
					}, // with injection status
					Labels: map[string]string{
						"kubeslice.io/pod-type": "app",
					}, // with pod type set
				},
				{
					Namespace: "kubeslice-system",
				}, // Deployed in control plane namespace
			}
			It("Should skip injection", func() {

				for _, meta := range table {
					is := deploy.MutationRequired(meta)
					Expect(is).To(BeFalse())
				}

			})
		})
	})

})
