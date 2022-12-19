package spoke_test

import (
	. "github.com/onsi/ginkgo/v2"
	_ "github.com/onsi/gomega"
)

// install workerslicegwrecyler crd
// setup initial test structure
// different tests

var _ = Describe("GW_Reblancing_FSM", func() {

	Context("With WorkerSliceGwRecyler created on worker cluster", func() {

		Context("SliceGw-Client", func() {
			It("should spin up new gw pod", func() {

			})
		})
		Context("SliceGw-Server", func() {
			It("should spin up new gw pod", func() {

			})
		})
	})

})
