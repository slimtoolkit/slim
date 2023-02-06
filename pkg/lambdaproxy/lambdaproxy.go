package lambdaproxy

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
)

var isbnRegexp = regexp.MustCompile(`[0-9]{3}\-[0-9]{10}`)
var errorLogger = log.New(os.Stderr, "ERROR ", log.Llongfile)

// HTTPProbeCmd provides the HTTP probe parameters
type HTTPProbeCmd struct {
	Method   string   `json:"method"`
	Resource string   `json:"resource"`
	Port     int      `json:"port"`
	Protocol string   `json:"protocol"`
	Headers  []string `json:"headers"`
	Body     string   `json:"body"`
	BodyFile string   `json:"body_file"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	Crawl    bool     `json:"crawl"`
}

type apiGatewayProxyRequest struct {
	Path                  string            `json:"path,omitempty"` // The url path for the caller
	HTTPMethod            string            `json:"httpMethod,omitempty"`
	Headers               map[string]string `json:"headers,omitempty"`
	QueryStringParameters map[string]string `json:"queryStringParameters,omitempty"`
	Body                  string            `json:"body,omitempty"`
	IsBase64Encoded       bool              `json:"isBase64Encoded,omitempty"`
	Resource              string            `json:"resource,omitempty"` // The resource path defined in API Gateway
}

type apiGatewayProxyResponse struct {
	StatusCode      int               `json:"statusCode"`
	Headers         map[string]string `json:"headers"`
	Body            string            `json:"body"`
	IsBase64Encoded bool              `json:"isBase64Encoded,omitempty"`
}

type HTTPRequest struct {
	Method   string   `json:"method"`
	Resource string   `json:"resource"`
	Headers  []string `json:"headers"`
	Body     string   `json:"body"`
	Protocol string   `json:"protocol"`
	Username string   `json:"username"`
	Password string   `json:"password"`
}

type HTTPResponse struct {
	StatusCode int      `json:"statusCode"`
	Headers    []string `json:"headers"`
	Body       string   `json:"body"`
}

// encode and decode options - future placeholder for v1 and v2 options of lambda results
type EncodeOptions struct {
	version string `json:"version"`
}

type DecodeOptions struct {
	version string `json:"version"`
}

// Add a helper for handling errors. This logs any error to os.Stderr
// and returns a 500 Internal Server Error response that the AWS API
// Gateway understands.
func serverError(err error) (apiGatewayProxyResponse, error) {
	errorLogger.Println(err.Error())

	return apiGatewayProxyResponse{
		StatusCode: http.StatusInternalServerError,
		Body:       http.StatusText(http.StatusInternalServerError),
	}, nil
}

// Similarly add a helper for send responses relating to client errors.
func clientError(status int) (apiGatewayProxyResponse, error) {
	return apiGatewayProxyResponse{
		StatusCode: status,
		Body:       http.StatusText(status),
	}, nil
}

func handleRequest(ctx context.Context, request *HTTPProbeCmd) (*HTTPRequest, error) {

	return &HTTPRequest{Method: request.Method, Resource: request.Resource, Headers: request.Headers, Body: request.Body}, nil
}

func EncodeRequest(input *HTTPRequest, options *EncodeOptions) ([]byte, error) {

	// encode http request to api gateway proxy request
	// to do matching of parsing of both structs
	encodeapiGatewayStruct := apiGatewayProxyRequest{
		Resource: input.Resource,
		Body:     input.Body,
		Headers:  convertSliceToMap(input.Headers),
	}

	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(&encodeapiGatewayStruct); err != nil {
		fmt.Errorf("Error encoding apiGatewayProxyRequest: %s", err)
		return nil, err
	}

	return b.Bytes(), nil
}

func DecodeResponse(input []byte, options *DecodeOptions) (HTTPResponse, error) {

	var response apiGatewayProxyResponse
	if err := json.NewDecoder(bytes.NewBuffer(input)).Decode(&response); err != nil {
		return HTTPResponse{}, err
	}
	//decode the response.Body if base64 encoded
	if response.IsBase64Encoded {
		responseBodyBytes, err := base64.StdEncoding.DecodeString(string(response.Body))
		if err != nil {
			log.Fatalf("Some error occured during base64 decode. Error %s", err.Error())
		}
		response.Body = string(responseBodyBytes)
	}

	return HTTPResponse{StatusCode: response.StatusCode, Body: response.Body, Headers: convertMapToSlice(response.Headers)}, nil
}

func convertSliceToMap(pairs []string) map[string]string {
	m := map[string]string{}
	for _, pair := range pairs {
		parts := strings.SplitN(pair, ":", 2)
		parts[0] = strings.TrimSpace(parts[0])
		parts[1] = strings.TrimSpace(parts[1])
		if val, found := m[parts[0]]; !found {
			m[parts[0]] = parts[1]
		} else {
			m[parts[0]] = fmt.Sprintf("%s,%s", val, parts[1])
		}
	}
	return m
}

func convertMapToSlice(input map[string]string) []string {
	// Convert map to slice of keys.
	//since map is unordered we need to sort the keys.
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Convert map to slice of values.
	values := []string{}
	for _, key := range keys {
		values = append(values, input[key])
	}

	// Convert map to slice of key-value pairs.
	pairs := []string{}
	for key, value := range input {
		splitvalue := strings.Split(value, ",")
		if len(splitvalue) > 1 {
			for _, v := range splitvalue {
				pairs = append(pairs, fmt.Sprintf("%s: %s", key, strings.TrimSpace(v)))
			}
		} else {
			pairs = append(pairs, fmt.Sprintf("%s: %s", key, value))
		}
	}

	return pairs
}
