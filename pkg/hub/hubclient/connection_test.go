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
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPrimaryConnection_MatchesTheConfiguredEnvironment is the no-regression
// test for passing connections around instead of reading the environment at the
// point of use: a worker with one hub must end up with exactly the endpoint and
// credentials it has always used.
func TestPrimaryConnection_MatchesTheConfiguredEnvironment(t *testing.T) {
	conn := PrimaryConnection()
	assert.Equal(t, HubEndpoint, conn.Endpoint)
	assert.Equal(t, HubTokenFile, conn.TokenFile)
	assert.Equal(t, HubCAFile, conn.CAFile)
}

// TestConnection_RestConfigCarriesTheWholeTriple guards the mistake the type
// exists to prevent. The endpoint used to be read late, inside the client
// builders, while HubTokenFile and HubCAFile are package vars fixed before main
// runs — so moving only the endpoint produced a client aimed at one hub
// authenticating as the other, which fails looking like a network problem.
func TestConnection_RestConfigCarriesTheWholeTriple(t *testing.T) {
	conn := Connection{
		Endpoint:  "https://hub-b.example:6443",
		TokenFile: "/creds/b/token",
		CAFile:    "/creds/b/ca.crt",
	}

	cfg := conn.RestConfig()
	assert.Equal(t, conn.Endpoint, cfg.Host)
	assert.Equal(t, conn.TokenFile, cfg.BearerTokenFile,
		"the token must follow the endpoint it belongs to")
	assert.Equal(t, conn.CAFile, cfg.TLSClientConfig.CAFile,
		"the CA must follow the endpoint it belongs to")
}
