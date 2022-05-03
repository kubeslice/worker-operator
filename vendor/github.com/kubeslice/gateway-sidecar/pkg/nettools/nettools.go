/*  Copyright (c) 2022 Avesha, Inc. All rights reserved.
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

package nettools

import (
	"fmt"
	"net"
	"strings"

	"github.com/pkg/errors"
)

// InterfaceInfo holds the information about the interface (Name and IP)
type InterfaceInfo struct {
	Name string
	IP   string
}

// GetPodIP provide the POD IP address
func GetPodIP() string {
	// Get interface addresses for all the interfaces
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	// Get the IP address for the interface addresses
	ipStr, _ := getIPForIfaceAddrs(addrs)
	return ipStr
}

// GetInterfaceIP provide the interface to get the IP address for given interface.
func GetInterfaceIP(ifaceName string) (string, error) {
	// Get the network interface details for the interface name provided
	ifaceVal, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return "", err
	}
	// Get interface addresses for the interface
	addrs, err := ifaceVal.Addrs()
	if err != nil {
		return "", err
	}
	// Get the IP address for the interface addresses
	ipStr, err := getIPForIfaceAddrs(addrs)
	return ipStr, err
}

// GetInterfaceInfos provide the interface information IP addresses and Interface names with interface name prefix.
func GetInterfaceInfos(ifaceNamePrefix string) ([]InterfaceInfo, error) {
	ipt := []InterfaceInfo{}

	// Get the interface list
	ift, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, ifi := range ift {
		// Check for the interface name prefix
		if strings.Contains(ifi.Name, ifaceNamePrefix) {
			// Get the interface address
			addrs, err := ifi.Addrs()
			if err != nil || len(addrs) == 0 {
				continue
			}
			// Get the IP address for the interface addresses
			ip, err := getIPForIfaceAddrs(addrs)
			if err != nil {
				continue
			}
			ipInfo := InterfaceInfo{
				ifi.Name,
				ip,
			}

			// Append to the list
			ipt = append(ipt, ipInfo)
		}
	}
	if len(ipt) == 0 {
		errStr := fmt.Sprintf("Couldn't find valid IP in the interface name %s prefix", ifaceNamePrefix)
		return nil, errors.New(errStr)
	}
	return ipt, nil
}

// getIPForIfaceAddrs is local function to get IP address for the interface addresses
func getIPForIfaceAddrs(addrs []net.Addr) (string, error) {

	for _, a := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}
	return "", errors.New("Couldn't find valid IP in the interface address")
}
