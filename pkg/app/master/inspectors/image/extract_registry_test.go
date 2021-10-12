package image

import (
	"reflect"
	"testing"
)

func TestRegistryExtraction(t *testing.T) {
	tt := []struct {
		in       string
		expected string
	}{
		{in: "https://gcr.io/nginx/nginx:3.9.11", expected: "https://gcr.io"},
		{in: "https://www.gcr.io/nginx/nginx:3.9.11", expected: "https://www.gcr.io"},
		{in: "eu.gcr.io/nginx/nginx:3.9.11", expected: "eu.gcr.io"},
		{in: "https://mcr.com/puppet/nginx:1.1.11", expected: "https://mcr.com"},
		{in: "http://192.168.10.11/nginx/nginx:3.9.11", expected: "http://192.168.10.11"},
		{in: "http://192.158.1.10:2678/nginx/nginx:3.9.11", expected: "http://192.158.1.10:2678"},
		{in: "https://192.158.1.10:2678/nginx/nginx:3.9.11", expected: "https://192.158.1.10:2678"},
		{in: "https://2001:0db8:85a3:0000:0000:8a2e:0370:7334/nginx/nginx:3.9.11", expected: "https://2001:0db8:85a3:0000:0000:8a2e:0370:7334"},
		{in: "https://2001:0db8:85a3:0000:0000:8a2e:0370:7334/nginx/nginx:3.9.11", expected: "https://2001:0db8:85a3:0000:0000:8a2e:0370:7334"},
		{in: "http://[2001:db8:1f70::999:de8:7648:6e8]:1000/nginx/rad:76.9", expected: "http://[2001:db8:1f70::999:de8:7648:6e8]:1000"},
		{in: "http://127.0.0.1/ops/scrap:latest", expected: "http://127.0.0.1"},
		{in: "127.0.0.1:4000/ops/scrap:latest", expected: "127.0.0.1:4000"},
		{in: "https://127.0.0.1:4000/ops/scrap:latest", expected: "https://127.0.0.1:4000"},
		{in: "http://localhost/ops/scrap:latest", expected: "http://localhost"},
		{in: "slim/docker-slim:latest", expected: "https://index.docker.io"},
		//{in: "local-registry/ops/scrap:latest", expected: "local-registry"},
		//{in: "local-registry:9000/ops/scrap:latest", expected: "local-registry:9000"},
	}

	for _, test := range tt {
		registry := extractRegistry(test.in)
		if !equal(registry, test.expected) {
			t.Errorf("got %s expected %s", registry, test.expected)
		}
	}
}

func equal(res, expected interface{}) bool {
	return reflect.DeepEqual(res, expected)
}
