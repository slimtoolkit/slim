// Copyright 2023 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package remote

import (
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// Referrers returns a list of descriptors that refer to the given manifest digest.
//
// The subject manifest doesn't have to exist in the registry for there to be descriptors that refer to it.
func Referrers(d name.Digest, options ...Option) (*v1.IndexManifest, error) {
	o, err := makeOptions(d.Context(), options...)
	if err != nil {
		return nil, err
	}
	f, err := makeFetcher(d, o)
	if err != nil {
		return nil, err
	}
	return f.fetchReferrers(o.context, o.filter, d)
}
