package lambdaproxy

import (
	"context"
	"testing"

	"gotest.tools/assert"
)

///WIP Test
func TestHandleRequest(t *testing.T) {

	// Create a new HTTPProbeCmd
	cmd := &HTTPProbeCmd{}

	// Call HandleRequest
	request, err := handleRequest(context.Background(), cmd)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the request is correct
	assert.Equal(t, request.Method, "GET")
	assert.Equal(t, request.Body, "Hello, World!")
	assert.Equal(t, request.Headers, []string{"Content-Type: text/plain"})

}
