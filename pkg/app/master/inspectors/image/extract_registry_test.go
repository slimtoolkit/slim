package image

import (
	"reflect"
	"testing"
)

func TestRegistryExtraction(t *testing.T) {
	tt := []struct {
		input string
		res   string
	}{
		{input: "https://gcr.io/nginx/nginx:3.9.11", res: "https://gcr.io"},
		{input: "https://www.gcr.io/nginx/nginx:3.9.11", res: "https://www.gcr.io"},
		{input: "eu.gcr.io/nginx/nginx:3.9.11", res: "eu.gcr.io"},
		{input: "https://mcr.com/puppet/nginx:1.1.11", res: "https://mcr.com"},
		{input: "http://192.168.10.11/nginx/nginx:3.9.11", res: "http://192.168.10.11"},
		{input: "http://192.158.1.10:2678/nginx/nginx:3.9.11", res: "http://192.158.1.10:2678"},
		{input: "https://192.158.1.10:2678/nginx/nginx:3.9.11", res: "https://192.158.1.10:2678"},
		{input: "https://2001:0db8:85a3:0000:0000:8a2e:0370:7334/nginx/nginx:3.9.11", res: "https://2001:0db8:85a3:0000:0000:8a2e:0370:7334"},
		{input: "https://2001:0db8:85a3:0000:0000:8a2e:0370:7334/nginx/nginx:3.9.11", res: "https://2001:0db8:85a3:0000:0000:8a2e:0370:7334"},
		{input: "http://[2001:db8:1f70::999:de8:7648:6e8]:1000/nginx/rad:76.9", res: "http://[2001:db8:1f70::999:de8:7648:6e8]:1000"},
		{input: "http://127.0.0.1/ops/scrap:latest", res: "http://127.0.0.1"},
		{input: "127.0.0.1:4000/ops/scrap:latest", res: "127.0.0.1:4000"},
		{input: "https://127.0.0.1:4000/ops/scrap:latest", res: "https://127.0.0.1:4000"},
		{input: "http://localhost/ops/scrap:latest", res: "http://localhost"},
		{input: "slim/docker-slim:latest", res: "https://index.docker.io"},
		//{input: "local-registry/ops/scrap:latest", res: "local-registry"},
		//{input: "local-registry:9000/ops/scrap:latest", res: "local-registry:9000"},
	}

	for _, test := range tt {
		res := extractRegistry(test.input)
		if !equal(extractRegistry(test.input), test.res) {
			t.Errorf("got %s expected %s", res, test.res)
		}
	}
}

func equal(res, expected interface{}) bool {
	return reflect.DeepEqual(res, expected)
}
