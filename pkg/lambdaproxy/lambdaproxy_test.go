package lambdaproxy

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	cmd     *HTTPProbeCmd
	request *HTTPRequest
)

// /WIP Test
func TestHandleRequest(t *testing.T) {

	// Create a new HTTPProbeCmd

	err := json.Unmarshal([]byte(`{
		"resource": "/2015-03-31/functions/function/invocations",
		"method": "POST",
		"body": "{\"httpMethod\":\"POST\", \"path\":\"/stuff\", \"body\":\"{\"key\":\"val data\"}\", \"headers\": {\"Content-Type\": \"application/json\"}}",
		"protocol": "http"
	}`), &cmd)

	if err != nil {
		t.Fatal(err)
	}

	// Call HandleRequest
	request, err = handleRequest(context.Background(), cmd)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the request is correct
	assert.Equal(t, request.Method, "POST")
	assert.Equal(t, request.Body, "{\"httpMethod\":\"POST\", \"path\":\"/stuff\", \"body\":\"{\"key\":\"val data\"}\", \"headers\": {\"Content-Type\": \"application/json\"}}")
	assert.Equal(t, request.Resource, "/2015-03-31/functions/function/invocations")
}

func TestEncodeRequest(t *testing.T) {

	var testrequest *HTTPRequest

	err := json.Unmarshal([]byte(`{
		"headers": [
			"header1: value1",
			"header2: value1,value2"
		],
		"body": "Hello from Lambda",
		"resource": "/2015-03-31/functions/function/invocations"
	}`), &testrequest)

	if err != nil {
		t.Fatal(err)
	}

	// Call EncodeRequest
	encodedRequest, err := EncodeRequest(testrequest, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the encoded request is correct
	//to-fix-this-test
	assert.Equal(t, string(encodedRequest),
		`{"headers":{"header1":"value1","header2":"value1,value2"},"body":"Hello from Lambda","resource":"/2015-03-31/functions/function/invocations"}
`)
}

// function to decode response from lambda proxy
func TestDecodeResponse(t *testing.T) {

	// Call DecodeResponse
	response, err := DecodeResponse([]byte(`{"statusCode":200,"headers":{"x-powered-by":"Express","content-type":"application/json; charset=utf-8","content-length":"68","etag":"W/\"44-ef4gTsYXxI2SCuYqkSK9GcCRgKo\""},"isBase64Encoded":false,"body":"{\"status\":\"ok\",\"call\":\"POST /stuff\",\"data\":\"{\\\"key\\\":\\\"val data\\\"}\"}"}`), &DecodeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// Check that the response is correct
	assert.Equal(t, response.StatusCode, 200)
	assert.Equal(t, response.Body, "{\"status\":\"ok\",\"call\":\"POST /stuff\",\"data\":\"{\\\"key\\\":\\\"val data\\\"}\"}")
	assert.ElementsMatch(t, response.Headers, []string{"x-powered-by: Express", "content-type: application/json; charset=utf-8", "content-length: 68", "etag: W/\"44-ef4gTsYXxI2SCuYqkSK9GcCRgKo\""})
}
