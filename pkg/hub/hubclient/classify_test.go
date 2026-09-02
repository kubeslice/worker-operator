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

package hub

import (
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/url"
	"testing"
)

func TestClassifyConnectionError_CertFailuresAreDistinguished(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"unknown authority", x509.UnknownAuthorityError{}},
		{"certificate invalid", x509.CertificateInvalidError{Reason: x509.Expired}},
		{"hostname mismatch", x509.HostnameError{Certificate: &x509.Certificate{}, Host: "hub.example"}},
		{"wrapped in a url.Error, as a real client-go transport failure would be",
			&url.Error{Op: "Get", URL: "https://hub.example", Err: x509.UnknownAuthorityError{}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ClassifyConnectionError(c.err)
			if got != ReasonCertVerificationFailed {
				t.Fatalf("ClassifyConnectionError(%v) = %q, want %q", c.err, got, ReasonCertVerificationFailed)
			}
		})
	}
}

func TestClassifyConnectionError_EverythingElseIsADialFailure(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"connection refused", &net.OpError{Op: "dial", Err: errors.New("connection refused")}},
		{"generic timeout", fmt.Errorf("context deadline exceeded")},
		{"not found, unrelated to connectivity", errors.New("clusters.kubeslice.io \"worker-1\" not found")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ClassifyConnectionError(c.err)
			if got != ReasonDialFailed {
				t.Fatalf("ClassifyConnectionError(%v) = %q, want %q", c.err, got, ReasonDialFailed)
			}
		})
	}
}
