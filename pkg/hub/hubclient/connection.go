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

import "k8s.io/client-go/rest"

// Connection is everything needed to reach one hub cluster: where it is, and
// the credentials for it. It travels together for a reason — the endpoint and
// the token are only valid as a pair, and a client built from one hub's address
// with the other hub's token fails authentication in a way that looks like a
// network problem.
//
// It is passed by value rather than read from the environment at the point of
// use, which matters once more than one hub is configured. The endpoint is read
// late (inside the functions that build a client) but HubTokenFile and
// HubCAFile are package-level vars initialised before main runs, so overriding
// the environment from main would move the address without moving the
// credentials. Passing the pair removes that whole class of mistake.
type Connection struct {
	Endpoint  string
	TokenFile string
	CAFile    string
}

// PrimaryConnection is the hub this worker was configured with, from the
// environment the deployment has always set. It is what a worker with no
// secondary hub configured uses, and is identical to what the client-building
// code read directly before connections were passed around.
func PrimaryConnection() Connection {
	return Connection{
		Endpoint:  HubEndpoint,
		TokenFile: HubTokenFile,
		CAFile:    HubCAFile,
	}
}

// RestConfig builds the client-go config for this hub.
func (c Connection) RestConfig() *rest.Config {
	return &rest.Config{
		Host:            c.Endpoint,
		BearerTokenFile: c.TokenFile,
		TLSClientConfig: rest.TLSClientConfig{
			CAFile: c.CAFile,
		},
	}
}
