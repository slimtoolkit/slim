# High Level Project Roadmap

This is a high level roadmap that identies the current areas of focus. Note that it's not a detailed list of every possible enhancement.

* Community
  * Collaborate with other CNCF projects to achieve mutually benefitial outcomes
  * Talks, outreach, community training
  * Engage with the community to increase project contributions

* Documentation
  * Improve system design documentations to make it easier for new contributors to contribute to the project
  * User docs (v1)

* Non-docker runtime support
  * Direct ContainerD support
  * Finch integration
  * Podman support
  * Kubernetes support vNext

* Container debugging
  * Ephemeral container based debugging for Kubernetes

* Build/Optimize engine
  * Error and logging enhancements to improve debuggability
  * Improved build flag documentation with examples
  * Improved CI/build tool integration documentation (including Github Actions)

* Integrations
  * Consign integrations for `xray` (reporting) and `build` (signing)

* Plugins
  * Plugin subsystem design
  * Sample plugins
  * Container image build plugin for BuildKit

* System sensor
  * System sensor subsystem design
  * External sensor integrations for Tetragon, Falco and Tracee as plugins

* Installers for all major platforms/package managers and publishing the packages to the official package manager distribution repos
  * Homebrew (official tap), Mac Ports
  * Apt
  * Yum/Dnf/Rpm
  * Apk
  * Aur
  * Nix

* Example
  * More build/optimize/minify examples
  * Documenting examples including the configs used to produce the minified images

