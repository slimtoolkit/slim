// Package buildpackinfo contains buildpack metadata extraction code
package buildpackinfo

const (
	LabelKeyStackID           = "io.buildpacks.stack.id"
	LabelKeyProjectMetadata   = "io.buildpacks.project.metadata"
	LabelKeyBuildMetadata     = "io.buildpacks.build.metadata"
	LabelKeyLifecycleMetadata = "io.buildpacks.lifecycle.metadata"
	LabelKeyStackMaintainer   = "io.buildpacks.stack.maintainer"
)

const (
	StackHeroku18 = "heroku-18"
	StackGoogle   = "google"
	StackPaketo   = "io.buildpacks.stacks.bionic"
)

const (
	VendorHeroku = "heroku"
	VendorGoogle = "google"
	VendorPaketo = "paketo"
)

const Entrypoint = "/cnb/process/web"

var Labels = map[string]struct{}{
	LabelKeyStackID:           {},
	LabelKeyProjectMetadata:   {},
	LabelKeyBuildMetadata:     {},
	LabelKeyLifecycleMetadata: {},
	LabelKeyStackMaintainer:   {},
}

func HasBuildbackLabels(labels map[string]string) bool {
	for k := range labels {
		if _, found := Labels[k]; found {
			return true
		}
	}

	return false
}
