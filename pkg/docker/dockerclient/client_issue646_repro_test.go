package dockerclient

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/slimtoolkit/slim/pkg/app/master/config"
)

// TestNewClientDefaultAPIVersion_Issue646 reproduces slimtoolkit/slim#646
// (the same bug as mintoolkit/mint#95 — ported one-to-one from
// w157-mint/pkg/crt/docker/dockerclient/client_issue95_repro_test.go,
// prep run #157/#158; slim's dockerclient.New() (pkg/docker/dockerclient/
// client.go) uses the exact same docker.NewVersionedClient(config.Host,
// config.APIVersion) + "SkipServerVersionCheck = true only when
// config.APIVersion != \"\"" logic as mint's pre-crt-refactor New(),
// confirmed by direct line-by-line comparison in prep run #154).
//
// When the caller does not set DOCKER_API_VERSION / config.APIVersion
// explicitly (the common case — a plain "slim build" from CI, every
// occurrence in the issue), dockerclient.New() builds a *docker.Client
// whose internal requestedAPIVersion stays nil for the whole life of the
// client. Because requestedAPIVersion is nil, every request that client
// sends is built WITHOUT a "/vX.Y/" path segment
// (vendor/github.com/fsouza/go-dockerclient/client.go:865-897). A Docker
// daemon that sits behind a proxy — dind (Docker-in-Docker: a container
// that itself runs a Docker daemon, the standard way Bitbucket/GitLab
// self-hosted CI runners give a pipeline Docker access without mounting
// the host socket) is the topology in the report — treats an unversioned
// request as coming from the oldest client it supports and rejects it
// with "client version ... is too old", the exact error text in #646.
//
// This test stands in a fake Docker daemon with httptest.Server (an
// in-process HTTP test server — no real Docker daemon needed) and records
// the path of the request dockerclient.New()'s client actually sends. It
// asserts the desired, fixed behavior: that the request path carries a
// version segment even when config.APIVersion was left empty by the
// caller. That is exactly what today's code does NOT do, so this test
// fails (red) against the unpatched client.New().
func TestNewClientDefaultAPIVersion_Issue646(t *testing.T) {
	var infoPath string
	sawInfoRequest := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/version" {
			// Answer the internal negotiation truthfully so checkAPIVersion()
			// succeeds and the client proceeds to the real /info call.
			_, _ = w.Write([]byte(`{"ApiVersion":"1.44"}`))
			return
		}
		infoPath = r.URL.Path
		sawInfoRequest = true
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	cfg := &config.DockerClient{
		Host:   srv.URL,
		UseTLS: false,
		// APIVersion intentionally left empty: this is the default,
		// undocumented-requirement path the #646 reporter hit.
		APIVersion: "",
	}

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("dockerclient.New() with empty APIVersion must succeed (it does for the real reporter): %v", err)
	}

	// Any request is enough to observe the path the client actually sends;
	// Info() is the simplest no-argument call go-dockerclient exposes.
	if _, err := client.Info(); err != nil {
		t.Fatalf("client.Info() must succeed against a daemon that answers /version correctly: %v", err)
	}

	if !sawInfoRequest {
		t.Fatalf("expected the fake daemon to receive the actual /info request after version negotiation, got none")
	}
	t.Logf("DEBUG infoPath = %q", infoPath)

	if !strings.Contains(infoPath, "/v") {
		t.Fatalf("slim#646: the /info request path %q carries no API version segment even though "+
			"config.APIVersion was left empty by the caller and the daemon answered the internal "+
			"/version negotiation just fine; a dind/CI daemon behind a proxy reads an unversioned "+
			"request as coming from the oldest supported client and rejects it with \"client version "+
			"... is too old\" (the exact error text reported in #646) — go-dockerclient negotiates the "+
			"server's version but never feeds the result back into requestedAPIVersion, which is the "+
			"only field getURL() consults", infoPath)
	}
}
