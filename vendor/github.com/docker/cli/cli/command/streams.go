package command

import (
	"github.com/docker/cli/cli/streams"
)

// InStream is an input stream used by the DockerCli to read user input
// Deprecated: Use github.com/docker/cli/cli/streams.In instead
type InStream = streams.In

// OutStream is an output stream used by the DockerCli to write normal program
// output.
// Deprecated: Use github.com/docker/cli/cli/streams.Out instead
type OutStream = streams.Out

var (
	// NewInStream returns a new InStream object from a ReadCloser
	// Deprecated: Use github.com/docker/cli/cli/streams.NewIn instead
	NewInStream = streams.NewIn
	// NewOutStream returns a new OutStream object from a Writer
	// Deprecated: Use github.com/docker/cli/cli/streams.NewOut instead
	NewOutStream = streams.NewOut
)
