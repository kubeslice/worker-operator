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

package sidecar

import (
	"fmt"
	"os"

	"github.com/kubeslice/gateway-sidecar/pkg/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	sliceGwTc              = TcInfo{}
	log                    = logger.NewLogger()
	dscpClassesList        = []string{"AF11", "AF12", "AF21", "AF22", "AF23", "AF31", "AF32", "AF33", "AF41", "AF42", "AF43", "EF"}
	dscpClassInUse  string = "Default"
)

func containsDscp(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}

// AF11
func sliceGwSetInterClusterDscpConfig(dscpClass string) error {
	if !containsDscp(dscpClassesList, dscpClass) {
		log.Infof("Dscp class string is not valid: %v", dscpClass)
		dscpClass = dscpClassInUse
	}

	portFilter := ""
	if os.Getenv("OPEN_VPN_MODE") == "CLIENT" {
		if SliceGwRemoteClusterNodePort == "" {
			log.Infof("Waiting for remote cluster node port to set the dscp config")
			return nil
		}
		portFilter = "--destination-port " + SliceGwRemoteClusterNodePort
	} else {
		portFilter = "--source-port 11194"
	}

	if dscpClass == dscpClassInUse {
		log.Infof("No change in DSCP marking needed: %v", dscpClassInUse)
		return nil
	}
	// Delete existing DSCP config before adding a new one
	if dscpClassInUse != "Default" {
		log.Infof("Updating DSCP marking from %v to %v", dscpClassInUse, dscpClass)
		ipTablesCmd := fmt.Sprintf("iptables -t mangle -D POSTROUTING -p udp %s -j DSCP --set-dscp-class %s",
			portFilter, dscpClassInUse)
		_, err := runCommand(ipTablesCmd)
		if err != nil {
			log.Errorf("Could not remove existing DSCP config: %v. DSCP class in use: %v", err, dscpClassInUse)
			return err
		}
		dscpClassInUse = "Default"
	}

	ipTablesCmd := fmt.Sprintf("iptables -t mangle -A POSTROUTING -p udp %s -j DSCP --set-dscp-class %s",
		portFilter, dscpClass)
	_, err := runCommand(ipTablesCmd)
	if err != nil {
		log.Errorf("DSCP marking failed: %v. DSCP class in use: %v", err, dscpClass)
		return err
	}
	dscpClassInUse = dscpClass

	return err
}

func sliceGwGetInterClusterDscpConfig() (string, error) {
	ipTablesCmd := "iptables -t mangle -n -L POSTROUTING"
	return runCommand(ipTablesCmd)
}

func (s *GwSidecar) enforceSliceGwTc(newTc TcInfo) error {
	if sliceGwTc == newTc {
		log.Infof("No change in TC params, ignoring update")
		return nil
	} else {
		log.Info("TC params updated. Old: %v, New: %v", sliceGwTc, newTc)
		tcCmd := fmt.Sprintf("tc qdisc delete dev tun0 root tbf rate %dkbit burst 32kbit latency 500ms", sliceGwTc.bwCeiling)
		_, err := runTcCommand(tcCmd)
		if err != nil {
			return status.Errorf(codes.Internal, "tc command %v execution failed: %v", tcCmd, err)
		}
	}

	// Add follow TC command
	// tc qdisc add dev tun0 root tbf rate 5mbit burst 32kbit latency 500ms
	tcCmd := fmt.Sprintf("tc qdisc add dev tun0 root tbf rate %dkbit burst 32kbit latency 500ms", newTc.bwCeiling)
	cmdOut, err := runTcCommand(tcCmd)
	if err != nil {
		return status.Errorf(codes.Internal, "tc command %v execution failed: %v", tcCmd, err)
	}
	sliceGwTc = newTc
	log.Infof("tc Command %v output :%v", tcCmd, cmdOut)

	tcCmd = "tc qdisc show dev tun0"
	cmdOut, err = runTcCommand(tcCmd)
	log.Infof("tc Command %v output :%v", tcCmd, cmdOut)

	return nil
}

func (s *GwSidecar) enforceInterClusterQosPolicy(dscpClass string) error {
	err := sliceGwSetInterClusterDscpConfig(dscpClass)
	if err != nil {
		return status.Errorf(codes.Internal, "Failed to configure DSCP on inter cluster traffic: %v", err)
	}

	dscpConfig, err := sliceGwGetInterClusterDscpConfig()
	log.Infof("DSCP setting for inter cluster traffic: %v", dscpConfig)

	return nil
}

func (s *GwSidecar) enforceSliceQosPolicy(qosProfile *SliceQosProfile) error {
	err := s.enforceSliceGwTc(TcInfo{
		class:        classType(qosProfile.GetClassType().String()),
		bwCeiling:    qosProfile.BwCeiling,
		bwGuaranteed: qosProfile.BwGuaranteed,
		priority:     qosProfile.Priority,
	})
	if err != nil {
		log.Errorf("Failed to enforce TC settings on sliceGw. err: %v", err)
	}

	err = s.enforceInterClusterQosPolicy(qosProfile.DscpClass)
	if err != nil {
		log.Errorf("Failed to enforce Inter Cluster QoS policy. err: %v", err)
	}

	return nil
}
