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

package failover

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// TestPackage_NeverImportsGatewayOrTunnelCode is issue #468 scenario 3
// (traffic-plane continuity), at the only level this package can actually
// prove it: gateways and tunnels run in their own pods and this package's
// job is to decide which hub to talk to and to ask the process to restart,
// nothing more. Live cross-cloud tunnel continuity during a real failover
// is separately blocked by an unrelated worker-CRD-skew defect and is not
// re-verified here — see docs/hub-failover.md.
func TestPackage_NeverImportsGatewayOrTunnelCode(t *testing.T) {
	banned := []string{
		"pkg/gwsidecar",
		"pkg/netop",
		"pkg/router",
		"pkg/gatewayedge",
		"pkg/slicegwrecycler",
		"hub/controllers/vpnkeyrotation",
		"hub/controllers/workerslicegwrecycler",
	}

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no source files found; the glob is broken, not the package")
	}

	fset := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		node, err := parser.ParseFile(fset, file, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parsing %s: %v", file, err)
		}
		for _, imp := range node.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			for _, b := range banned {
				if strings.Contains(path, b) {
					t.Errorf("%s imports %q, which owns gateway/tunnel pods; the failover-following"+
						" package must stay confined to deciding which hub to talk to", file, path)
				}
			}
		}
	}
}
