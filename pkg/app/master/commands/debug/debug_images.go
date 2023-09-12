package debug

const (
	CgrSlimToolkitDebugImage = "cgr.dev/chainguard/slim-toolkit-debug:latest"
	WolfiBaseImage           = "cgr.dev/chainguard/wolfi-base:latest"
	BusyboxImage             = "busybox:latest"
	NicolakaNetshootImage    = "nicolaka/netshoot"
	KoolkitsNodeImage        = "lightruncom/koolkits:node"
	KoolkitsPythonImage      = "lightruncom/koolkits:python"
	KoolkitsGolangImage      = "lightruncom/koolkits:golang"
	KoolkitsJVMImage         = "lightruncom/koolkits:jvm"
	DigitaloceanDoksImage    = "digitalocean/doks-debug:latest"
	ZinclabsUbuntuImage      = "public.ecr.aws/zinclabs/debug-ubuntu-base:latest"
	InfuserImage             = "ghcr.io/teaxyz/infuser:latest"
)

var debugImages = map[string]string{
	CgrSlimToolkitDebugImage: "Chainguard SlimToolkit debug image - https://edu.chainguard.dev/chainguard/chainguard-images/reference/slim-toolkit-debug",
	WolfiBaseImage:           "A lightweight Wolfi base image - https://github.com/chainguard-images/images/tree/main/images/wolfi-base",
	BusyboxImage:             "A lightweight image with common unix utilities - https://busybox.net/about.html",
	NicolakaNetshootImage:    "Network trouble-shooting swiss-army container - https://github.com/nicolaka/netshoot",
	KoolkitsNodeImage:        "Node.js KoolKit - https://github.com/lightrun-platform/koolkits/tree/main/nodejs",
	KoolkitsPythonImage:      "Python KoolKit - https://github.com/lightrun-platform/koolkits/tree/main/python",
	KoolkitsGolangImage:      "Go KoolKit - https://github.com/lightrun-platform/koolkits/tree/main/golang",
	KoolkitsJVMImage:         "JVM KoolKit - https://github.com/lightrun-platform/koolkits/blob/main/jvm/README.md",
	DigitaloceanDoksImage:    "Kubernetes manifests for investigation and troubleshooting - https://github.com/digitalocean/doks-debug",
	ZinclabsUbuntuImage:      "Common utilities for debugging your cluster - https://github.com/openobserve/debug-container",
	InfuserImage:             "Tea package manager image - https://github.com/teaxyz/infuser",
}
