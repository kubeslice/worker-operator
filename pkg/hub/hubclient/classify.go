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
)

// Connection error reasons, matching worker-operator issue #469's
// ControllerConnected reason table. DialFailed is the default for anything
// that isn't specifically a certificate problem: a refused connection, a DNS
// failure, and a timeout all look the same to an operator deciding what to
// check first, and all point at network/endpoint configuration rather than
// the CA bundle.
const (
	ReasonDialFailed             = "DialFailed"
	ReasonCertVerificationFailed = "CertVerificationFailed"
)

// ClassifyConnectionError reports which of the two reasons a hub-connection
// error matches. err must be non-nil.
func ClassifyConnectionError(err error) string {
	var unknownAuthority x509.UnknownAuthorityError
	var certInvalid x509.CertificateInvalidError
	var hostnameErr x509.HostnameError
	if errors.As(err, &unknownAuthority) || errors.As(err, &certInvalid) || errors.As(err, &hostnameErr) {
		return ReasonCertVerificationFailed
	}
	return ReasonDialFailed
}
