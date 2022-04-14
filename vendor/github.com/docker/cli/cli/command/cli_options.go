package command

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/docker/cli/cli/context/docker"
	"github.com/docker/cli/cli/context/store"
	"github.com/docker/cli/cli/streams"
	"github.com/moby/term"
)

// DockerCliOption applies a modification on a DockerCli.
type DockerCliOption func(cli *DockerCli) error

// WithStandardStreams sets a cli in, out and err streams with the standard streams.
func WithStandardStreams() DockerCliOption {
	return func(cli *DockerCli) error {
		// Set terminal emulation based on platform as required.
		stdin, stdout, stderr := term.StdStreams()
		cli.in = streams.NewIn(stdin)
		cli.out = streams.NewOut(stdout)
		cli.err = stderr
		return nil
	}
}

// WithCombinedStreams uses the same stream for the output and error streams.
func WithCombinedStreams(combined io.Writer) DockerCliOption {
	return func(cli *DockerCli) error {
		cli.out = streams.NewOut(combined)
		cli.err = combined
		return nil
	}
}

// WithInputStream sets a cli input stream.
func WithInputStream(in io.ReadCloser) DockerCliOption {
	return func(cli *DockerCli) error {
		cli.in = streams.NewIn(in)
		return nil
	}
}

// WithOutputStream sets a cli output stream.
func WithOutputStream(out io.Writer) DockerCliOption {
	return func(cli *DockerCli) error {
		cli.out = streams.NewOut(out)
		return nil
	}
}

// WithErrorStream sets a cli error stream.
func WithErrorStream(err io.Writer) DockerCliOption {
	return func(cli *DockerCli) error {
		cli.err = err
		return nil
	}
}

// WithContentTrustFromEnv enables content trust on a cli from environment variable DOCKER_CONTENT_TRUST value.
func WithContentTrustFromEnv() DockerCliOption {
	return func(cli *DockerCli) error {
		cli.contentTrust = false
		if e := os.Getenv("DOCKER_CONTENT_TRUST"); e != "" {
			if t, err := strconv.ParseBool(e); t || err != nil {
				// treat any other value as true
				cli.contentTrust = true
			}
		}
		return nil
	}
}

// WithContentTrust enables content trust on a cli.
func WithContentTrust(enabled bool) DockerCliOption {
	return func(cli *DockerCli) error {
		cli.contentTrust = enabled
		return nil
	}
}

// WithContextEndpointType add support for an additional typed endpoint in the context store
// Plugins should use this to store additional endpoints configuration in the context store
func WithContextEndpointType(endpointName string, endpointType store.TypeGetter) DockerCliOption {
	return func(cli *DockerCli) error {
		switch endpointName {
		case docker.DockerEndpoint:
			return fmt.Errorf("cannot change %q endpoint type", endpointName)
		}
		cli.contextStoreConfig.SetEndpoint(endpointName, endpointType)
		return nil
	}
}
