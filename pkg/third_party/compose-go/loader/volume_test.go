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
	"fmt"
	"testing"

	"github.com/compose-spec/compose-go/types"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestParseVolumeAnonymousVolume(t *testing.T) {
	for _, path := range []string{"/path", "/path/foo"} {
		volume, err := ParseVolume(path)
		expected := types.ServiceVolumeConfig{Type: "volume", Target: path, Volume: &types.ServiceVolumeVolume{}}
		assert.NilError(t, err)
		assert.Check(t, is.DeepEqual(expected, volume))
	}
}

func TestParseVolumeAnonymousVolumeWindows(t *testing.T) {
	for _, path := range []string{"C:\\path", "Z:\\path\\foo"} {
		volume, err := ParseVolume(path)
		expected := types.ServiceVolumeConfig{Type: "volume", Target: path, Volume: &types.ServiceVolumeVolume{}}
		assert.NilError(t, err)
		assert.Check(t, is.DeepEqual(expected, volume))
	}
}

func TestParseVolumeTooManyColons(t *testing.T) {
	_, err := ParseVolume("/foo:/foo:ro:foo")
	assert.Error(t, err, "invalid spec: /foo:/foo:ro:foo: too many colons")
}

func TestParseVolumeShortVolumes(t *testing.T) {
	for _, path := range []string{".", "/a"} {
		volume, err := ParseVolume(path)
		expected := types.ServiceVolumeConfig{Type: "volume", Target: path}
		assert.NilError(t, err)
		assert.Check(t, is.DeepEqual(expected, volume))
	}
}

func TestParseVolumeMissingSource(t *testing.T) {
	for _, spec := range []string{":foo", "/foo::ro"} {
		_, err := ParseVolume(spec)
		assert.ErrorContains(t, err, "empty section between colons")
	}
}

func TestParseVolumeBindMount(t *testing.T) {
	for _, path := range []string{"./foo", "~/thing", "../other", "/foo", "/home/user"} {
		volume, err := ParseVolume(path + ":/target")
		expected := types.ServiceVolumeConfig{
			Type:   "bind",
			Source: path,
			Target: "/target",
			Bind:   &types.ServiceVolumeBind{CreateHostPath: true},
		}
		assert.NilError(t, err)
		assert.Check(t, is.DeepEqual(expected, volume))
	}
}

func TestParseVolumeRelativeBindMountWindows(t *testing.T) {
	for _, path := range []string{
		"./foo",
		"~/thing",
		"../other",
		"D:\\path", "/home/user",
	} {
		volume, err := ParseVolume(path + ":d:\\target")
		expected := types.ServiceVolumeConfig{
			Type:   "bind",
			Source: path,
			Target: "d:\\target",
			Bind:   &types.ServiceVolumeBind{CreateHostPath: true},
		}
		assert.NilError(t, err)
		assert.Check(t, is.DeepEqual(expected, volume))
	}
}

func TestParseVolumeWithBindOptions(t *testing.T) {
	volume, err := ParseVolume("/source:/target:slave")
	expected := types.ServiceVolumeConfig{
		Type:   "bind",
		Source: "/source",
		Target: "/target",
		Bind: &types.ServiceVolumeBind{
			CreateHostPath: true,
			Propagation:    "slave",
		},
	}
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(expected, volume))
}

func TestParseVolumeWithBindOptionsWindows(t *testing.T) {
	volume, err := ParseVolume("C:\\source\\foo:D:\\target:ro,rprivate")
	expected := types.ServiceVolumeConfig{
		Type:     "bind",
		Source:   "C:\\source\\foo",
		Target:   "D:\\target",
		ReadOnly: true,
		Bind: &types.ServiceVolumeBind{
			CreateHostPath: true,
			Propagation:    "rprivate",
		},
	}
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(expected, volume))
}

func TestParseVolumeWithInvalidVolumeOptions(t *testing.T) {
	_, err := ParseVolume("name:/target:bogus")
	assert.NilError(t, err)
}

func TestParseVolumeWithVolumeOptions(t *testing.T) {
	volume, err := ParseVolume("name:/target:nocopy")
	expected := types.ServiceVolumeConfig{
		Type:   "volume",
		Source: "name",
		Target: "/target",
		Volume: &types.ServiceVolumeVolume{NoCopy: true},
	}
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(expected, volume))
}

func TestParseVolumeWithReadOnly(t *testing.T) {
	for _, path := range []string{"./foo", "/home/user"} {
		volume, err := ParseVolume(path + ":/target:ro")
		expected := types.ServiceVolumeConfig{
			Type:     "bind",
			Source:   path,
			Target:   "/target",
			ReadOnly: true,
			Bind:     &types.ServiceVolumeBind{CreateHostPath: true},
		}
		assert.NilError(t, err)
		assert.Check(t, is.DeepEqual(expected, volume))
	}
}

func TestParseVolumeWithRW(t *testing.T) {
	for _, path := range []string{"./foo", "/home/user"} {
		volume, err := ParseVolume(path + ":/target:rw")
		expected := types.ServiceVolumeConfig{
			Type:     "bind",
			Source:   path,
			Target:   "/target",
			ReadOnly: false,
			Bind:     &types.ServiceVolumeBind{CreateHostPath: true},
		}
		assert.NilError(t, err)
		assert.Check(t, is.DeepEqual(expected, volume))
	}
}

func TestParseVolumeWindowsNamedPipe(t *testing.T) {
	volume, err := ParseVolume(`\\.\pipe\docker_engine:\\.\pipe\inside`)
	assert.NilError(t, err)
	expected := types.ServiceVolumeConfig{
		Type:   "bind",
		Source: `\\.\pipe\docker_engine`,
		Target: `\\.\pipe\inside`,
		Bind:   &types.ServiceVolumeBind{CreateHostPath: true},
	}
	assert.Check(t, is.DeepEqual(expected, volume))
}

func TestIsFilePath(t *testing.T) {
	assert.Check(t, !isFilePath("a界"))
}

// Preserve the test cases for VolumeSplitN
func TestParseVolumeSplitCases(t *testing.T) {
	for casenumber, x := range []struct {
		input    string
		n        int
		expected []string
	}{
		{`C:\foo:d:`, -1, []string{`C:\foo`, `d:`}},
		{`:C:\foo:d:`, -1, nil},
		{`/foo:/bar:ro`, 3, []string{`/foo`, `/bar`, `ro`}},
		{`/foo:/bar:ro`, 2, []string{`/foo`, `/bar:ro`}},
		{`C:\foo\:/foo`, -1, []string{`C:\foo\`, `/foo`}},
		{`d:\`, -1, []string{`d:\`}},
		{`d:`, -1, []string{`d:`}},
		{`d:\path`, -1, []string{`d:\path`}},
		{`d:\path with space`, -1, []string{`d:\path with space`}},
		{`d:\pathandmode:rw`, -1, []string{`d:\pathandmode`, `rw`}},

		{`c:\:d:\`, -1, []string{`c:\`, `d:\`}},
		{`c:\windows\:d:`, -1, []string{`c:\windows\`, `d:`}},
		{`c:\windows:d:\s p a c e`, -1, []string{`c:\windows`, `d:\s p a c e`}},
		{`c:\windows:d:\s p a c e:RW`, -1, []string{`c:\windows`, `d:\s p a c e`, `RW`}},
		{`c:\program files:d:\s p a c e i n h o s t d i r`, -1, []string{`c:\program files`, `d:\s p a c e i n h o s t d i r`}},
		{`0123456789name:d:`, -1, []string{`0123456789name`, `d:`}},
		{`MiXeDcAsEnAmE:d:`, -1, []string{`MiXeDcAsEnAmE`, `d:`}},
		{`name:D:`, -1, []string{`name`, `D:`}},
		{`name:D::rW`, -1, []string{`name`, `D:`, `rW`}},
		{`name:D::RW`, -1, []string{`name`, `D:`, `RW`}},

		{`c:/:d:/forward/slashes/are/good/too`, -1, []string{`c:/`, `d:/forward/slashes/are/good/too`}},
		{`c:\Windows`, -1, []string{`c:\Windows`}},
		{`c:\Program Files (x86)`, -1, []string{`c:\Program Files (x86)`}},
		{``, -1, nil},
		{`.`, -1, []string{`.`}},
		{`..\`, -1, []string{`..\`}},
		{`c:\:..\`, -1, []string{`c:\`, `..\`}},
		{`c:\:d:\:xyzzy`, -1, []string{`c:\`, `d:\`, `xyzzy`}},
		// Cover directories with one-character name
		{`/tmp/x/y:/foo/x/y`, -1, []string{`/tmp/x/y`, `/foo/x/y`}},
	} {
		parsed, _ := ParseVolume(x.input)

		expected := len(x.expected) > 1
		msg := fmt.Sprintf("Case %d: %s", casenumber, x.input)
		assert.Check(t, is.Equal(expected, parsed.Source != ""), msg)
	}
}

func TestParseVolumeInvalidEmptySpec(t *testing.T) {
	_, err := ParseVolume("")
	assert.ErrorContains(t, err, "invalid empty volume spec")
}

func TestParseVolumeInvalidSections(t *testing.T) {
	_, err := ParseVolume("/foo::rw")
	assert.ErrorContains(t, err, "invalid spec")
}
