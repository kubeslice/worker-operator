package manifest_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kubeslice/operator/internal/manifest"
	appsv1 "k8s.io/api/apps/v1"
)

var _ = Describe("Manifest File", func() {

	Context("With k8s deployment manifest", func() {

		f := "egress-deploy"

		It("Should parse file into k8s deployment in the slice", func() {

			m := manifest.NewManifest(f, "green")

			Expect(m).ToNot(BeNil())

			deploy := &appsv1.Deployment{}
			err := m.Parse(deploy)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploy).ToNot(BeNil())

			Expect(deploy.ObjectMeta.Name).Should(Equal("green-istio-egressgateway"))

		})

	})

})
