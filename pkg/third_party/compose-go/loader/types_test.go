/*
   Copyright 2020 The Compose Specification Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package loader

import (
	"encoding/json"
	"os"
	"testing"

	yaml "gopkg.in/yaml.v2"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestMarshallConfig(t *testing.T) {
	workingDir, err := os.Getwd()
	assert.NilError(t, err)
	homeDir, err := os.UserHomeDir()
	assert.NilError(t, err)
	cfg := fullExampleConfig(workingDir, homeDir)
	expected := fullExampleYAML(workingDir, homeDir)

	actual, err := yaml.Marshal(cfg)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(expected, string(actual)))

	// Make sure the expected still
	_, err = Load(buildConfigDetails(expected, map[string]string{}), func(options *Options) {
		options.SkipNormalization = true
		options.SkipConsistencyCheck = true
	})
	assert.NilError(t, err)
}

func TestJSONMarshallConfig(t *testing.T) {
	workingDir, err := os.Getwd()
	assert.NilError(t, err)
	homeDir, err := os.UserHomeDir()
	assert.NilError(t, err)

	cfg := fullExampleConfig(workingDir, homeDir)
	expected := fullExampleJSON(workingDir, homeDir)

	actual, err := json.MarshalIndent(cfg, "", "  ")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(expected, string(actual)))

	_, err = Load(buildConfigDetails(expected, map[string]string{}), func(options *Options) {
		options.SkipNormalization = true
		options.SkipConsistencyCheck = true
	})
	assert.NilError(t, err)
}
