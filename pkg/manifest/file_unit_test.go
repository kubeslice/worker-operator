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
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetManifestPath(t *testing.T) {
	tests := []struct {
		name         string
		file         string
		manifestPath string
		expected     string
	}{
		{
			name:         "with MANIFEST_PATH set",
			file:         "test-file",
			manifestPath: "/custom/path",
			expected:     "/custom/path/test-file.json",
		},
		{
			name:         "without MANIFEST_PATH set",
			file:         "test-file",
			manifestPath: "",
			expected:     "../../files/manifests/test-file.json",
		},
		{
			name:         "with different file name",
			file:         "ingress-deploy",
			manifestPath: "/another/path",
			expected:     "/another/path/ingress-deploy.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.manifestPath != "" {
				os.Setenv("MANIFEST_PATH", tt.manifestPath)
				defer os.Unsetenv("MANIFEST_PATH")
			} else {
				os.Unsetenv("MANIFEST_PATH")
			}

			result := GetManifestPath(tt.file)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewManifest(t *testing.T) {
	tests := []struct {
		name      string
		file      string
		templates map[string]string
	}{
		{
			name: "create manifest with templates",
			file: "test-file",
			templates: map[string]string{
				"SLICE": "test-slice",
				"IMAGE": "test-image",
			},
		},
		{
			name:      "create manifest without templates",
			file:      "test-file",
			templates: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewManifest(tt.file, tt.templates)
			assert.NotNil(t, result)
			assert.Equal(t, GetManifestPath(tt.file), result.Path)
			assert.Equal(t, tt.templates, result.Templates)
		})
	}
}

func TestManifestParse(t *testing.T) {
	tests := []struct {
		name          string
		fileContent   string
		templates     map[string]string
		expectedError bool
	}{
		{
			name:        "parse valid JSON with template",
			fileContent: `{"name": "SLICE-deployment", "replicas": 3}`,
			templates: map[string]string{
				"SLICE": "test-slice",
			},
			expectedError: false,
		},
		{
			name:        "parse JSON with multiple templates",
			fileContent: `{"name": "SLICE-deployment", "image": "IMAGE"}`,
			templates: map[string]string{
				"SLICE": "test-slice",
				"IMAGE": "test-image",
			},
			expectedError: false,
		},
		{
			name:          "parse invalid JSON",
			fileContent:   `{invalid json}`,
			templates:     map[string]string{},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := ioutil.TempDir("", "manifest-test")
			assert.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			tmpFile := path.Join(tmpDir, "test.json")
			err = ioutil.WriteFile(tmpFile, []byte(tt.fileContent), 0644)
			assert.NoError(t, err)

			m := &Manifest{
				Path:      tmpFile,
				Templates: tt.templates,
			}

			var result map[string]interface{}
			err = m.Parse(&result)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestManifestParseFileNotFound(t *testing.T) {
	m := &Manifest{
		Path:      "/non/existent/path/file.json",
		Templates: map[string]string{},
	}

	var result map[string]interface{}
	err := m.Parse(&result)
	assert.Error(t, err)
}

func TestManifestParseTemplateReplacement(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "manifest-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	fileContent := `{"name": "SLICE-deployment", "namespace": "NAMESPACE", "replicas": 3}`
	tmpFile := path.Join(tmpDir, "test.json")
	err = ioutil.WriteFile(tmpFile, []byte(fileContent), 0644)
	assert.NoError(t, err)

	templates := map[string]string{
		"SLICE":     "my-slice",
		"NAMESPACE": "my-namespace",
	}

	m := &Manifest{
		Path:      tmpFile,
		Templates: templates,
	}

	var result map[string]interface{}
	err = m.Parse(&result)
	assert.NoError(t, err)

	assert.Equal(t, "my-slice-deployment", result["name"])
	assert.Equal(t, "my-namespace", result["namespace"])
	assert.Equal(t, json.Number("3"), result["replicas"])
}
