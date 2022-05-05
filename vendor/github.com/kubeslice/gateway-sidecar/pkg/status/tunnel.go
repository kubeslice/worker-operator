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

package status

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-ping/ping"
	"github.com/kubeslice/gateway-sidecar/pkg/cmd"
	"github.com/kubeslice/gateway-sidecar/pkg/exec"
	"github.com/kubeslice/gateway-sidecar/pkg/logger"
	"github.com/kubeslice/gateway-sidecar/pkg/metrics"
	"github.com/kubeslice/gateway-sidecar/pkg/nettools"
	"github.com/pkg/errors"
)

// msgType defines possible status types.
type msgType uint

// Constant to define the Tunnel Message Type
const (
	UpdatePeerIP  msgType = 1 << iota // Update PeerIP
	RestartPinger                     // Restart Pinger
)

// tunnelMessage
type tunnelMessage struct {
	ty  msgType
	msg interface{}
}

// TunnelChecker represents
type TunnelChecker struct {
	log       *logger.Logger
	exMod     *exec.Module
	tunStatus *TunnelInterfaceStatus
	pinger    *ping.Pinger
	txBytes   uint64
	rxBytes   uint64
	startTime int64
}

// NewTunnelChecker returns a Check that pings using the specified Pinger and fails on timeout or ping failure
func NewTunnelChecker(log *logger.Logger) Check {
	return &TunnelChecker{
		log:       log,
		exMod:     nil,
		tunStatus: nil,
		txBytes:   0,
		rxBytes:   0,
		startTime: 0,
	}
}

// Execute executes the Tunnel status check
func (t *TunnelChecker) Execute(interface{}) (err error) {
	ifaceInfos, err := nettools.GetInterfaceInfos("tun")
	if err != nil {
		t.tunStatus = nil
		t.log.Errorf("Unable to find the tun interface")
		return err
	}
	if len(ifaceInfos) > 1 || len(ifaceInfos) == 0 {
		t.tunStatus = nil
		t.log.Errorf("Invalid tunnel interface")
		return errors.New("Invalid tunnel interface")
	}
	if t.tunStatus == nil {
		t.tunStatus = new(TunnelInterfaceStatus)
		t.startTime = getCurTimeMs()
	}
	//add metrics which can be shown on prometheus
	metrics.RecordLatencyMetric(float64(t.tunStatus.Latency))
	metrics.RecordRxRateMetric(float64(t.tunStatus.RxRate))
	metrics.RecordTxRateMetric(float64(t.tunStatus.TxRate))

	t.tunStatus.NetInterface = ifaceInfos[0].Name
	t.tunStatus.LocalIP = ifaceInfos[0].IP
	t.updateNetworkStatus(ifaceInfos[0].Name)
	return nil
}

// Status to provide the Tunnel status
func (t *TunnelChecker) Status() (details interface{}, err error) {
	if t.tunStatus == nil {
		return nil, errors.New("Tunnel is not Up")
	}
	return t.tunStatus, nil
}

// Stop the status of the check
func (t *TunnelChecker) Stop() error {
	if t.pinger != nil {
		// Stop the Pinger
		t.pinger.Stop()
		t.log.Infof("Ping Stopped")
	}

	t.exMod.Stop()
	t.log.Infof("TunnelChecker Stopped")
	t.exMod = nil
	t.tunStatus = nil
	return nil
}

// MessageHandler to provide the Tunnel status
func (t *TunnelChecker) MessageHandler(msg interface{}) error {
	tunMsg := msg.(*tunnelMessage)

	switch tunMsg.ty {
	case UpdatePeerIP:
		t.log.Debugf("PeerIP Message %v ", t.tunStatus.PeerIP)

		t.tunStatus.PeerIP = fmt.Sprintf("%v", tunMsg.msg)
		if t.pinger != nil {
			// Stop the Pinger
			t.pinger.Stop()
			t.pinger = nil
		}
		err := t.startPing(t.tunStatus.PeerIP)
		if err != nil {
			t.log.Errorf("Ping Start failed: %s", err.Error())
			return err
		}
		t.log.Debugf("Ping Started")
	case RestartPinger:
		err := t.startPing(t.tunStatus.PeerIP)
		if err != nil {
			t.log.Errorf("Ping Re-Start failed: %s", err.Error())
			return err
		}
		t.log.Debugf("Ping Re-Started")
	}
	return nil
}

// UpdateExecModule updates the exec module
func (t *TunnelChecker) UpdateExecModule(e *exec.Module) {
	t.exMod = e
}

// UpdatePeerIP updates the peer IP
func (t *TunnelChecker) UpdatePeerIP(peerIP string) error {
	if t.tunStatus == nil {
		t.log.Error("Tunnel Not Created Yet")
		return errors.Errorf("Tunnel is not up yet..")
	}
	t.exMod.SendMsg(&tunnelMessage{UpdatePeerIP, peerIP})
	return nil
}

// startPing starts the ping process
func (t *TunnelChecker) startPing(host string) error {
	// t.log.Infof("Starting the ping check to :%v", host)
	pinger, err := ping.NewPinger(host)
	if err != nil {
		t.log.Errorf("Pinger Create failed: %s", err.Error())
		return err
	}
	t.pinger = pinger
	pinger.Count = 3
	pinger.Interval = time.Second
	pinger.Timeout = 3 * time.Second
	pinger.SetPrivileged(true)
	pinger.OnFinish = t.onFinishCb

	go pinger.Run()

	return nil
}

// onFinishCb is called when ping is finished.
func (t *TunnelChecker) onFinishCb(stats *ping.Statistics) {
	if t.tunStatus != nil {
		t.tunStatus.Latency = uint64(stats.AvgRtt / time.Millisecond)
		t.log.Debugf("Latency :%v", t.tunStatus.Latency)
		if t.exMod != nil {
			t.exMod.SendMsg(&tunnelMessage{ty: RestartPinger, msg: nil})
		}
	}
}

// updateNetworkStatus updates the network status (TxRate and RxRate)
func (t *TunnelChecker) updateNetworkStatus(ifaceName string) error {
	txCmd := fmt.Sprintf("cat /sys/class/net/%s/statistics/tx_bytes", ifaceName)
	rxCmd := fmt.Sprintf("cat /sys/class/net/%s/statistics/rx_bytes", ifaceName)
	var txBytes, rxBytes uint64 = 0, 0
	cmdOut, err := cmd.Run(txCmd)
	if err != nil {
		errStr := fmt.Sprintf("Command: %v execution failed with err: %v and stderr : %v", txCmd, err, cmdOut)
		t.log.Errorf(errStr)
	} else {
		txBytes, err = strconv.ParseUint(strings.TrimSuffix(cmdOut, "\n"), 10, 64)
		if err != nil {
			t.log.Errorf("txBytes integer conversion failed %v", err)
		}
	}
	t.log.Debugf("Command: %v output :%v", txCmd, cmdOut)

	cmdOut, err = cmd.Run(rxCmd)
	if err != nil {
		errStr := fmt.Sprintf("Command: %v execution failed with err: %v and stderr : %v", rxCmd, err, cmdOut)
		t.log.Errorf(errStr)
	} else {
		rxBytes, _ = strconv.ParseUint(strings.TrimSuffix(cmdOut, "\n"), 10, 64)
	}
	t.log.Debugf("Command: %v output :%v", rxCmd, cmdOut)

	curTime := getCurTimeMs()
	timeDelta := curTime - t.startTime
	t.log.Debugf("Current time: %v Start Time : %v timeDelta: %v prev txBytes: %v prev rxBytes: %v cur txBytes: %v cur rxBytes: %v", curTime, t.startTime, timeDelta, t.txBytes, t.rxBytes, txBytes, rxBytes)
	if (txBytes - t.txBytes) < 0 {
		t.log.Errorf("Negative txBytes ")
		return nil
	}
	if (rxBytes - t.rxBytes) < 0 {
		t.log.Errorf("Negative rxBytes ")
		return nil
	}

	t.tunStatus.TxRate = uint64(((txBytes - t.txBytes) / uint64(timeDelta)) * 8)
	t.tunStatus.RxRate = uint64(((rxBytes - t.rxBytes) / uint64(timeDelta)) * 8)
	t.log.Infof("TxRate: %v RxRate: %v", t.tunStatus.TxRate, t.tunStatus.RxRate)
	t.txBytes = txBytes
	t.rxBytes = rxBytes
	t.startTime = getCurTimeMs()
	return nil
}

// getCurTimeMs is function to get current time in milliseconds.
func getCurTimeMs() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
