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
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/google/shlex"
	"github.com/kubeslice/gateway-sidecar/pkg/nettools"
	"github.com/kubeslice/gateway-sidecar/pkg/status"
)

const (
	nsmInterfaceName string = "nsm0"
)

var (
	statusMonitor *status.Monitor
)

func SetStatusMonitor(sm *status.Monitor) {
	statusMonitor = sm
}

func getGwPodStatus() (*GwPodStatus, error) {
	podStatus := &GwPodStatus{}
	podStatus.GatewayPodIP = nettools.GetPodIP()
	podStatus.NodeIP = os.Getenv("NODE_IP")
	podNsmIP, err := nettools.GetInterfaceIP(nsmInterfaceName)
	if err != nil {
		podNsmIP = ""
	}

	nsmIntfStatus := NsmInterfaceStatus{
		NsmInterfaceName: nsmInterfaceName,
		NsmIP:            podNsmIP,
	}

	podStatus.NsmIntfStatus = &nsmIntfStatus

	if statusMonitor != nil {
		// Get the monitor status checks
		checks := statusMonitor.Checks()
		for _, v := range checks {
			stats, err := v.Status()
			if err != nil {
				continue
			}
			tunStat := *stats.(*status.TunnelInterfaceStatus)
			tunnelStatus := TunnelInterfaceStatus{
				NetInterface: tunStat.NetInterface,
				LocalIP:      tunStat.LocalIP,
				PeerIP:       tunStat.PeerIP,
				Latency:      tunStat.Latency,
				TxRate:       tunStat.TxRate,
				RxRate:       tunStat.RxRate,
			}
			podStatus.TunnelStatus = &tunnelStatus
		}
	}
	return podStatus, nil
}

// runCommand runs the command string
func runCommand(cmdString string) (string, error) {
	var outb, errb bytes.Buffer

	ss, err := shlex.Split(cmdString)
	if err != nil {
		errMsg := fmt.Sprintf("Command split failed with error : %v", err)
		return "", errors.New(errMsg)
	}
	if len(ss) == 0 {
		errMsg := fmt.Sprintf("No command defined : %v", cmdString)
		return "", errors.New(errMsg)
	}
	cmd := exec.Command(ss[0], ss[1:]...)
	if err != nil {
		errMsg := fmt.Sprintf("Command construction failed with error : %v", err)
		return "", errors.New(errMsg)
	}
	cmd.Stdout = &outb
	cmd.Stderr = &errb

	// Run the command
	err = cmd.Run()
	if err != nil {
		errMsg := fmt.Sprintf("Could not run cmd: %v", err)
		return errb.String(), errors.New(errMsg)

	}
	return outb.String(), nil
}

func runTcCommand(tcCmd string) (string, error) {
	var errVal error = nil
	var err error = nil
	var cmdOut string = ""
	cmdOut, err = runCommand(tcCmd)
	if err != nil {
		errStr := fmt.Sprintf("tc Command: %v execution failed with err: %v and stderr : %v", tcCmd, err, cmdOut)
		log.Errorf(errStr)

		if strings.Contains(cmdOut, "RTNETLINK answers: File exists") {
			tcDelCmd := strings.Replace(tcCmd, "add", "del", -1)
			cmdOut, err = runCommand(tcDelCmd)
			if err != nil {
				errStr := fmt.Sprintf("tc Command: %v execution failed with err: %v and stderr : %v", tcDelCmd, err, cmdOut)
				log.Errorf(errStr)
				errVal = errors.New(errStr)
			}
			log.Debugf("tc Command: %v output :%v", tcDelCmd, cmdOut)

			// Re run the tc command
			cmdOut, err = runCommand(tcCmd)
			if err != nil {
				errStr := fmt.Sprintf("tc Command: %v execution failed with err: %v and stderr : %v", tcCmd, err, cmdOut)
				errVal = errors.New(errStr)
			}
			log.Infof("tc Command: %v output :%v", tcCmd, cmdOut)
		}
	}
	return cmdOut, errVal
}

func updateGwStatusWithConContext(conContext *SliceGwConnectionContext) error {
	log.Infof("conContext : %v", conContext)
	var errVal error = nil

	for k, v := range statusMonitor.Checks() {
		switch k {
		case "TunnelCheck":
			if conContext.GetRemoteSliceGwVpnIP() == "" {
				errVal = errors.New("invalid Remote Slice Gateway VPN IP")
			} else {
				if err := v.(*status.TunnelChecker).UpdatePeerIP(conContext.GetRemoteSliceGwVpnIP()); err != nil {
					return err
				}
			}
		}
	}
	return errVal
}
