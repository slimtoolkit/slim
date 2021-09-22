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

package schema

import (
	"testing"

	"gotest.tools/v3/assert"
)

type dict map[string]interface{}

func TestValidate(t *testing.T) {
	config := dict{
		"services": dict{
			"foo": dict{
				"image": "busybox",
			},
		},
	}

	assert.NilError(t, Validate(config))
}

func TestValidateUndefinedTopLevelOption(t *testing.T) {
	config := dict{
		"helicopters": dict{
			"foo": dict{
				"image": "busybox",
			},
		},
	}

	err := Validate(config)
	assert.ErrorContains(t, err, "Additional property helicopters is not allowed")
}

func TestValidateAllowsXTopLevelFields(t *testing.T) {
	config := dict{
		"x-extra-stuff": dict{},
	}

	assert.NilError(t, Validate(config))
	assert.NilError(t, Validate(config))
}

func TestValidateAllowsXFields(t *testing.T) {
	config := dict{
		"services": dict{
			"bar": dict{
				"x-extra-stuff": dict{},
			},
		},
		"volumes": dict{
			"bar": dict{
				"x-extra-stuff": dict{},
			},
		},
		"networks": dict{
			"bar": dict{
				"x-extra-stuff": dict{},
			},
		},
		"configs": dict{
			"bar": dict{
				"x-extra-stuff": dict{},
			},
		},
		"secrets": dict{
			"bar": dict{
				"x-extra-stuff": dict{},
			},
		},
	}
	assert.NilError(t, Validate(config))
	assert.NilError(t, Validate(config))
}

func TestValidateSecretConfigNames(t *testing.T) {
	config := dict{
		"configs": dict{
			"bar": dict{
				"name": "foobar",
			},
		},
		"secrets": dict{
			"baz": dict{
				"name": "foobaz",
			},
		},
	}

	assert.NilError(t, Validate(config))
	assert.NilError(t, Validate(config))
}

type array []interface{}

func TestValidatePlacement(t *testing.T) {
	config := dict{
		"services": dict{
			"foo": dict{
				"image": "busybox",
				"deploy": dict{
					"placement": dict{
						"preferences": array{
							dict{
								"spread": "node.labels.az",
							},
						},
					},
				},
			},
		},
	}

	assert.NilError(t, Validate(config))
	assert.NilError(t, Validate(config))
}

func TestValidateIsolation(t *testing.T) {
	config := dict{
		"services": dict{
			"foo": dict{
				"image":     "busybox",
				"isolation": "some-isolation-value",
			},
		},
	}
	assert.NilError(t, Validate(config))
	assert.NilError(t, Validate(config))
}

func TestValidateRollbackConfig(t *testing.T) {
	config := dict{
		"services": dict{
			"foo": dict{
				"image": "busybox",
				"deploy": dict{
					"rollback_config": dict{
						"parallelism": 1,
					},
				},
			},
		},
	}

	assert.NilError(t, Validate(config))
	assert.NilError(t, Validate(config))
}

func TestValidateRollbackConfigWithOrder(t *testing.T) {
	config := dict{
		"services": dict{
			"foo": dict{
				"image": "busybox",
				"deploy": dict{
					"rollback_config": dict{
						"parallelism": 1,
						"order":       "start-first",
					},
				},
			},
		},
	}

	assert.NilError(t, Validate(config))
	assert.NilError(t, Validate(config))
}

func TestValidateRollbackConfigWithUpdateConfig(t *testing.T) {
	config := dict{
		"services": dict{
			"foo": dict{
				"image": "busybox",
				"deploy": dict{
					"update_config": dict{
						"parallelism": 1,
						"order":       "start-first",
					},
					"rollback_config": dict{
						"parallelism": 1,
						"order":       "start-first",
					},
				},
			},
		},
	}

	assert.NilError(t, Validate(config))
	assert.NilError(t, Validate(config))
}

func TestValidateRollbackConfigWithUpdateConfigFull(t *testing.T) {
	config := dict{
		"services": dict{
			"foo": dict{
				"image": "busybox",
				"deploy": dict{
					"update_config": dict{
						"parallelism":    1,
						"order":          "start-first",
						"delay":          "10s",
						"failure_action": "pause",
						"monitor":        "10s",
					},
					"rollback_config": dict{
						"parallelism":    1,
						"order":          "start-first",
						"delay":          "10s",
						"failure_action": "pause",
						"monitor":        "10s",
					},
				},
			},
		},
	}

	assert.NilError(t, Validate(config))
	assert.NilError(t, Validate(config))
}
