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

package types

import (
	"testing"

	"gotest.tools/v3/assert"
)

func Test_WithServices(t *testing.T) {
	p := Project{
		Services: append(Services{},
			ServiceConfig{
				Name: "service_1",
				DependsOn: map[string]ServiceDependency{
					"service_3": {
						Condition: ServiceConditionStarted,
					},
				},
			}, ServiceConfig{
				Name: "service_2",
			}, ServiceConfig{
				Name: "service_3",
				DependsOn: map[string]ServiceDependency{
					"service_2": {
						Condition: ServiceConditionStarted,
					},
				},
			}),
	}
	order := []string{}
	fn := func(service ServiceConfig) error {
		order = append(order, service.Name)
		return nil
	}

	err := p.WithServices(nil, fn)
	assert.NilError(t, err)
	assert.DeepEqual(t, order, []string{"service_2", "service_3", "service_1"})
}
