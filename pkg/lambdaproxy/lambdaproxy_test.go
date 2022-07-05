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
	assert.Equal(t, request.Body, "{\"status\":\"ok\",\"call\":\"GET /hello\",\"data\":\"custom UA\"}")
	assert.Equal(t, request.Headers, []string{`"x-powered-by": "Express", "content-type": "application/json; charset=utf-8", "content-length": "68", "etag": "W/\"44-ef4gTsYXxI2SCuYqkSK9GcCRgKo\""`})
}

func TestEncodeRequest(t *testing.T) {
	// Create a new HTTPProbeCmd
	cmd := &HTTPProbeCmd{}

	// Call HandleRequest
	request, err := handleRequest(context.Background(), cmd)
	if err != nil {
		t.Fatal(err)
	}

	// Call EncodeRequest
	encodedRequest, err := EncodeRequest(request, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the encoded request is correct
	//to-do

	assert.Equal(t, encodedRequest, "")
}

//function to decode responsd from lambda proxy
func TestDecodeResponse(t *testing.T) {

	// Create a new HTTPProbeCmd
	cmd := &HTTPProbeCmd{}

	// Call HandleRequest
	request, err := handleRequest(context.Background(), cmd)
	if err != nil {
		t.Fatal(err)
	}

	// Call EncodeRequest
	encodedRequest, err := EncodeRequest(request, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Call DecodeResponse
	response, err := DecodeResponse([]byte(encodedRequest), &DecodeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// Check that the response is correct
	assert.Equal(t, response.StatusCode, 200)
	assert.Equal(t, response.Body, "{\"status\":\"ok\",\"call\":\"GET /hello\",\"data\":\"custom UA\"}")
	assert.Equal(t, response.Headers, []string{`"x-powered-by": "Express", "content-type": "application/json; charset=utf-8", "content-length": "68", "etag": "W/\"44-ef4gTsYXxI2SCuYqkSK9GcCRgKo\""`})
}
