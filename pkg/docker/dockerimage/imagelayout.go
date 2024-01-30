package dockerimage

const (
	dockerManifestFileName = "manifest.json"
	dockerReposFileName    = "repositories"

	//dockerV1 config object file name: <IMAGE_ID>.json
	dockerV1LayerSuffix = "/layer.tar"
)

const (
	ociLayoutFileName = "oci-layout"
	ociLayoutVersion  = "1.0.0"
	ociIndexFileName  = "index.json"
	ociBlobDirName    = "blobs"
	ociBlobDirPrefix  = "blobs/"
)

type OCILayout struct {
	Version string `json:"imageLayoutVersion"`
}
