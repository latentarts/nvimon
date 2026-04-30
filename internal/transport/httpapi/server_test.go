package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prods/nvimon/internal/collector"
)

func TestSnapshotEndpoint(t *testing.T) {
	c := collector.NewSampleCollector("host-a", "gpu-a", time.Second)
	server := NewServer(c, "")

	client := &Client{
		BaseURL: "http://nvimon.test",
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				rec := httptest.NewRecorder()
				server.Handler().ServeHTTP(rec, req)
				return recorderResponse(rec), nil
			}),
		},
	}

	snapshot, err := client.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	if snapshot.HostID != "host-a" {
		t.Fatalf("host id = %q, want host-a", snapshot.HostID)
	}

	if snapshot.GPUBackend != "sample" {
		t.Fatalf("gpu backend = %q, want sample", snapshot.GPUBackend)
	}
}

func TestSnapshotEndpointRequiresToken(t *testing.T) {
	c := collector.NewSampleCollector("host-a", "gpu-a", time.Second)
	server := NewServer(c, "secret")

	req := httptest.NewRequest(http.MethodGet, "/v1/snapshot", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHealthzEndpoint(t *testing.T) {
	c := collector.NewSampleCollector("host-a", "gpu-a", time.Second)
	server := NewServer(c, "")

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode healthz body: %v", err)
	}

	if body["status"] != "ok" {
		t.Fatalf("status body = %+v, want ok", body)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func recorderResponse(rec *httptest.ResponseRecorder) *http.Response {
	return &http.Response{
		StatusCode: rec.Code,
		Header:     rec.Header(),
		Body:       io.NopCloser(strings.NewReader(rec.Body.String())),
	}
}
