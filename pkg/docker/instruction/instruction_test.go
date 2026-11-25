package instruction

import (
	"strings"
	"testing"
)

func TestTypeConstantsAndIsKnown(t *testing.T) {
	pairs := []struct {
		lower string
		upper string
	}{
		{Add, TypeAdd},
		{Arg, TypeArg},
		{Cmd, TypeCmd},
		{Copy, TypeCopy},
		{Entrypoint, TypeEntrypoint},
		{Env, TypeEnv},
		{Expose, TypeExpose},
		{From, TypeFrom},
		{Healthcheck, TypeHealthcheck},
		{Label, TypeLabel},
		{Maintainer, TypeMaintainer},
		{Onbuild, TypeOnbuild},
		{Run, TypeRun},
		{Shell, TypeShell},
		{StopSignal, TypeStopSignal},
		{User, TypeUser},
		{Volume, TypeVolume},
		{Workdir, TypeWorkdir},
	}

	for _, p := range pairs {
		if strings.ToUpper(p.lower) != p.upper {
			t.Errorf("expected ToUpper(%q) == %q, got %q", p.lower, p.upper, strings.ToUpper(p.lower))
		}
		if !IsKnown(p.lower) {
			t.Errorf("IsKnown(%q) = false, want true", p.lower)
		}
		if !IsKnown(p.upper) {
			t.Errorf("IsKnown(%q) = false, want true", p.upper)
		}
	}
}
