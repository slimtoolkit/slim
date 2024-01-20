package consts

// Labels added to optimized container images
const (
	DSLabelVersion           = "slimtoolkit.version"
	DSLabelSourceImage       = "slimtoolkit.source.image"
	DSLabelSourceImageID     = "slimtoolkit.source.image.id"
	DSLabelSourceImageDigest = "slimtoolkit.source.image.digest"
)

// Other constants that external users/consumers will see
const (
	//reverse engineered Dockerfile for the target container image
	ReversedDockerfile        = "Dockerfile.reversed"
	ReversedDockerfileOldName = "Dockerfile.fat" //tmp compat
)
