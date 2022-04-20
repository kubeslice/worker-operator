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

package manifest

import (
	// corev1 "k8s.io/api/core/v1"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"encoding/json"

	"bitbucket.org/realtimeai/kubeslice-operator/internal/logger"
)

var log = logger.NewLogger()

func GetManifestPath(file string) string {
	dir := os.Getenv("MANIFEST_PATH")
	if dir != "" {
		return path.Join(dir, file+".json")
	}

	return path.Join("../../files/manifests", file+".json")
}

type Manifest struct {
	Slice string
	Path  string
}

func NewManifest(f string, slice string) *Manifest {
	return &Manifest{
		Path:  GetManifestPath(f),
		Slice: slice,
	}
}

func (m *Manifest) Parse(v interface{}) error {
	jsonFile, err := ioutil.ReadFile(m.Path)
	if err != nil {
		log.Error(err, "unable to read json file")
		return err
	}

	f := strings.ReplaceAll(string(jsonFile), "SLICE", m.Slice)

	err = json.Unmarshal([]byte(f), v)
	if err != nil {
		log.Error(err, "unable to parse json file as Deployment")
		return err
	}

	return nil

}
