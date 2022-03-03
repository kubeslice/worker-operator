package spoke_test

import (
	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("SliceController", func() {

	Context("With a Slice CR created", func() {
		It("Should create the slice", func() {

			slice := &meshv1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "kubeslice-system",
				},
				Spec: meshv1beta1.SliceSpec{},
			}

			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())

		})

	})

})
