package manifest_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bitbucket.org/realtimeai/kubeslice-operator/internal/manifest"
)

var _ = Describe("Manifest File", func() {

	Context("With k8s deployment manifest", func() {

		f := "../../tests/files/manifests/ingress-deploy.yaml"

		It("Should parse file into k8s deployment", func() {

			m := manifest.NewManifest(f)

			Expect(m).ToNot(BeNil())

			d, err := m.ParseDeployment()
			Expect(err).NotTo(HaveOccurred())
			Expect(d).ToNot(BeNil())

      Expect(d.ObjectMeta.Name).Should(Equal("istio-ingressgateway"))

		})

	})

})
