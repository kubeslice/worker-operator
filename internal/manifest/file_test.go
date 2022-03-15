package manifest_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bitbucket.org/realtimeai/kubeslice-operator/internal/manifest"
	appsv1 "k8s.io/api/apps/v1"
)

var _ = Describe("Manifest File", func() {

	Context("With k8s deployment manifest", func() {

		f := "../../tests/files/manifests/ingress-deploy.json"

		It("Should parse file into k8s deployment", func() {

			m := manifest.NewManifest(f)

			Expect(m).ToNot(BeNil())

			deploy := &appsv1.Deployment{}
			err := m.Parse(deploy)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploy).ToNot(BeNil())

			Expect(deploy.ObjectMeta.Name).Should(Equal("istio-ingressgateway"))

		})

	})

})
