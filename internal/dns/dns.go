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

package dns

import (
	"context"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/internal/logger"
)

// ReconcileDNSFile reconciles dns file in configmap with the available endpoints in serviceimport
func ReconcileDNSFile(ctx context.Context, dnsFile string, si *kubeslicev1beta1.ServiceImport) (string, error) {
	log := logger.FromContext(ctx).WithValues("type", "CoreDNS")
	debugLog := log.V(1)

	debugLog.Info("Reconciling DNS file", "dnsFile", dnsFile)

	endpoints := si.Status.Endpoints

	df := strings.Split(dnsFile, "\n")
	dnsEntries := df[1:]
	debugLog.Info("DNS entries", "size", len(dnsEntries), "SOA", df[0], "entries", dnsEntries)

	dnsEntries = removeServiceDnsRecords(dnsEntries, si)

	entriesForSi := []string{}

	for _, e := range endpoints {
		// make sure dnsName is present
		if e.DNSName == "" {
			continue
		}

		d1 := CreateDNSEntry(e.DNSName, e.IP)
		d2 := CreateDNSEntry(si.Spec.DNSName, e.IP)
		entriesForSi = append(entriesForSi, d1, d2)
	}

	dnsEntries = append(dnsEntries, entriesForSi...)
	sort.Strings(dnsEntries)

	if !reflect.DeepEqual(dnsEntries, df[1:]) {
		// If dns entries were modified, generate the entire file with new serial number in SOA
		u := GenerateDNSFile(df[0], dnsEntries)
		log.Info("dns records updated")
		debugLog.Info("dns records updated", "entries", dnsEntries, "dnsFile", u)
		return u, nil
	}

	return dnsFile, nil

}

func DeleteRecordsAndReconcileDNSFile(ctx context.Context, dnsFile string, si *kubeslicev1beta1.ServiceImport) (string, error) {
	log := logger.FromContext(ctx).WithValues("type", "CoreDNS")
	debugLog := log.V(1)

	log.Info("Deleting records for service import", "ServiceImport", si.Name)

	debugLog.Info("Deleting records in DNS file", "dnsFile", dnsFile, "recordsToDelete", si.Status.Endpoints)

	df := strings.Split(dnsFile, "\n")
	dnsEntries := df[1:]

	dnsEntries = removeServiceDnsRecords(dnsEntries, si)
	sort.Strings(dnsEntries)

	u := GenerateDNSFile(df[0], dnsEntries)
	log.Info("dns records deleted")
	debugLog.Info("dns records updated", "entries", dnsEntries, "dnsFile", u)

	return u, nil

}

func removeServiceDnsRecords(dnsEntries []string, si *kubeslicev1beta1.ServiceImport) []string {
	de := []string{}
	for _, v := range dnsEntries {
		dnsName := strings.Split(v, ". ")[0]
		if dnsName == si.Spec.DNSName {
			continue
		}
		if strings.HasSuffix(dnsName, "."+si.Name+"."+si.Namespace+".svc.slice.local") {
			continue
		}

		// ingress pods
		if strings.HasPrefix(dnsName, si.Name+"-ingress") && strings.HasSuffix(dnsName, si.Namespace+".svc.slice.local") {
			continue
		}
		de = append(de, v)
	}

	return de
}

// GenerateDNSFile Takes SOA header, list of dns A record strings and return as a single string
func GenerateDNSFile(header string, dnsEntries []string) string {
	headerParams := strings.Split(header, " ")
	currentTime := time.Now().Unix()

	if len(headerParams) == 10 {
		// If not in correct format, just ignore and return the same header
		headerParams[5] = strconv.FormatInt(currentTime, 10)
	}

	newDNSFile := strings.Join(headerParams, " ")

	// Append SOA to the beginning and return the entries as a single file
	if len(dnsEntries) > 0 {
		newDNSFile = newDNSFile + "\n" + strings.Join(dnsEntries, "\n")
	}

	return newDNSFile

}

// CreateDNSEntry converts endpoint object to DNS A record
func CreateDNSEntry(dnsName string, ip string) string {
	return dnsName + ". IN A " + ip
}
