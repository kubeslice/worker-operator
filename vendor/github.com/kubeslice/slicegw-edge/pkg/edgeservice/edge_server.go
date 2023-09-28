package edgeservice

import (
	"context"
	"fmt"
	"strings"

	"github.com/kubeslice/slicegw-edge/pkg/logger"

	"github.com/coreos/go-iptables/iptables"
)

var (
	log *logger.Logger = logger.NewLogger()
)

type GwEdgeService struct {
	UnimplementedGwEdgeServiceServer
}

type serviceInfo struct {
	svcIP      string
	nodePort   uint32
	targetPort uint32
}

// TODO: Change this to sync.Map
var serviceMap map[string]serviceInfo

func updateNeeded(svcList []*SliceGwServiceInfo) bool {
	if len(serviceMap) != len(svcList) {
		return true
	}

	for _, svcInfo := range svcList {
		if updateNeededForSvc(svcInfo.GwSvcName,
			serviceInfo{
				svcIP:      svcInfo.GwSvcClusterIP,
				nodePort:   svcInfo.GwSvcNodePort,
				targetPort: svcInfo.GwSvcTargetPort,
			}) {
			return true
		}
	}

	return false
}

func updateNeededForSvc(svcName string, svcInfo serviceInfo) bool {
	cachedInfo, found := serviceMap[svcName]
	if !found {
		return true
	}

	if cachedInfo.svcIP != svcInfo.svcIP || cachedInfo.nodePort != svcInfo.nodePort || cachedInfo.targetPort != svcInfo.targetPort {
		return true
	}

	return false
}

func deleteIpTablesRule(svcInfo serviceInfo) error {
	ipt, err := iptables.NewWithProtocol(iptables.ProtocolIPv4)
	if err != nil {
		log.Error(err, "Failed to init iptables handle")
		return err
	}

	rulespec := fmt.Sprintf("-p udp --dport %d -j DNAT --to-destination %s:%d", svcInfo.nodePort, svcInfo.svcIP, svcInfo.targetPort)
	err = ipt.DeleteIfExists("nat", "PREROUTING", strings.Split(rulespec, " ")...)
	if err != nil {
		return err
	}

	return nil
}

func deleteIpTablesRuleForSvc(svcName string) error {
	cachedInfo, found := serviceMap[svcName]
	if !found {
		return nil
	}

	err := deleteIpTablesRule(cachedInfo)
	if err != nil {
		return err
	}

	delete(serviceMap, svcName)

	return nil
}

func addIpTablesRuleForSvc(svcName string, svcInfo serviceInfo) error {
	ipt, err := iptables.NewWithProtocol(iptables.ProtocolIPv4)
	if err != nil {
		log.Error(err, "Failed to init iptables handle")
		return err
	}

	// Be careful (learned it the hard way) while drafting the rulespec. Even a single unwanted, benign
	// space will result in failed iptables API call.
	rulespec := fmt.Sprintf("-p udp --dport %d -j DNAT --to-destination %s:%d", svcInfo.nodePort, svcInfo.svcIP, svcInfo.targetPort)
	err = ipt.AppendUnique("nat", "PREROUTING", strings.Split(rulespec, " ")...)
	if err != nil {
		log.Error(err, "Failed to add iptables rule", "rulespec", rulespec)
		return nil
	}

	serviceMap[svcName] = svcInfo

	return nil
}

func (s *GwEdgeService) UpdateSliceGwServiceMap(ctx context.Context, in *SliceGwServiceMap) (*GwEdgeResponse, error) {
	log.Info("Received update from operator", "servicemap", in.SliceGwServiceList)

	if serviceMap == nil {
		serviceMap = make(map[string]serviceInfo)
	}

	if !updateNeeded(in.SliceGwServiceList) {
		return &GwEdgeResponse{StatusMsg: "Success"}, nil
	}

	// Check if any rule needs to be deleted. Service info present in cache but missing in the update message.
	for svcName, cachedInfo := range serviceMap {
		found := false
		for _, svcInfo := range in.SliceGwServiceList {
			if svcInfo.GwSvcName == svcName {
				found = true
				break
			}
		}

		if !found {
			err := deleteIpTablesRuleForSvc(svcName)
			if err != nil {
				log.Error(err, "Failed to remove iptables rule for svc", "svcinfo", cachedInfo)
				return &GwEdgeResponse{StatusMsg: "Failed to update iptables"}, err
			}
		}
	}

	for _, sliceGwSvcInfo := range in.SliceGwServiceList {
		nodePort := sliceGwSvcInfo.GwSvcNodePort
		targetPort := sliceGwSvcInfo.GwSvcTargetPort
		svcIP := sliceGwSvcInfo.GwSvcClusterIP

		// Check if an update is needed for this svc
		if !updateNeededForSvc(sliceGwSvcInfo.GwSvcName, serviceInfo{
			svcIP:      svcIP,
			nodePort:   nodePort,
			targetPort: targetPort,
		}) {
			continue
		}

		// Delete if rule is present for this svc
		err := deleteIpTablesRuleForSvc(sliceGwSvcInfo.GwSvcName)
		if err != nil {
			log.Error(err, "Failed to remove iptables rule for svc", "svcName", sliceGwSvcInfo.GwSvcName)
			return &GwEdgeResponse{StatusMsg: "Failed to update iptables"}, err
		}

		err = addIpTablesRuleForSvc(sliceGwSvcInfo.GwSvcName, serviceInfo{
			svcIP:      svcIP,
			nodePort:   nodePort,
			targetPort: targetPort,
		})
		if err != nil {
			log.Error(err, "Failed to add iptables rule", "svcName", sliceGwSvcInfo.GwSvcName)
			return &GwEdgeResponse{StatusMsg: "Failed to add rule to iptables"}, err
		}
	}

	return &GwEdgeResponse{StatusMsg: "Success"}, nil
}
