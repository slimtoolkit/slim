package builder

import (
	"context"
	"path/filepath"

	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
)

// ImageBuilder builds an image and returns some data from the build.
type ImageBuilder interface {
	// Build the image.
	Build(context.Context) error
	// GetLogs from the image builder.
	GetLogs() string
	// HasData file/dir that got ADD'd or COPY'd into the image, respectively.
	HasData() bool
}

const (
	tarData = "files.tar"
	dirData = "files"
)

// getDataName returns a predetermined name if it exists within the root dir.
func getDataName(root string) string {

	dataTar := filepath.Join(root, tarData)
	dataDir := filepath.Join(root, dirData)
	if fsutil.IsRegularFile(dataTar) {
		return tarData
	} else if fsutil.IsDir(dataDir) {
		return dirData
	}

	return ""
}
