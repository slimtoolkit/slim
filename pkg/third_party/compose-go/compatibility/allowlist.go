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

package compatibility

import (
	"fmt"

	"github.com/compose-spec/compose-go/errdefs"
	"github.com/pkg/errors"
)

// AllowList implements the Checker interface by rejecting all attributes that are not listed as "supported".
type AllowList struct {
	Supported []string
	errors    []error
}

// Errors returns the list of errors encountered when checking against the allow list
func (c *AllowList) Errors() []error {
	return c.errors
}

func (c *AllowList) supported(attributes ...string) bool {
	for _, a := range attributes {
		for _, s := range c.Supported {
			if s == a {
				return true
			}
		}
	}
	return false
}

func (c *AllowList) Unsupported(message string, args ...interface{}) {
	c.errors = append(c.errors, errors.Wrap(errdefs.ErrUnsupported, fmt.Sprintf(message, args...)))
}

func (c *AllowList) Incompatible(message string, args ...interface{}) {
	c.errors = append(c.errors, errors.Wrap(errdefs.ErrIncompatible, fmt.Sprintf(message, args...)))
}
