package tui

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/prods/nvimon/internal/collector"
	"github.com/prods/nvimon/internal/model"
	"github.com/prods/nvimon/internal/transport/httpapi"
)

type stubHostSource struct {
	name     string
	snapshot model.HostSnapshot
	err      error
}

func (s stubHostSource) Name() string {
	return s.name
}

func (s stubHostSource) Fetch(context.Context) (model.HostSnapshot, error) {
	return s.snapshot, s.err
}

func TestFetchSnapshotsCmdMixedSources(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	msg := fetchSnapshotsCmd([]hostSource{
		stubHostSource{
			name: "local",
			snapshot: model.HostSnapshot{
				HostID:       "local",
				Hostname:     "local",
				GPUBackend:   "sample",
				Timestamp:    now,
				CPUUsedPct:   model.NewMetricValue(10),
				GPUs:         []model.GPUSnapshot{{Index: 0, UUID: "GPU-0"}},
				GPUProcesses: []model.GPUProcess{{PID: 1}},
			},
		},
		stubHostSource{
			name: "remote",
			err:  errors.New("timeout"),
		},
	}, time.Second)().(snapshotsMsg)

	if len(msg.results) != 2 {
		t.Fatalf("result count = %d, want 2", len(msg.results))
	}
	if msg.results[0].snapshot.GPUBackend != "sample" {
		t.Fatalf("backend = %q, want sample", msg.results[0].snapshot.GPUBackend)
	}
	if msg.results[1].err == nil {
		t.Fatal("expected remote error")
	}
}

func TestModelUpdateMixedSourcesAndSelection(t *testing.T) {
	now := time.Unix(200, 0).UTC()
	m := New([]hostSource{
		stubHostSource{name: "local"},
		stubHostSource{name: "remote"},
	}, time.Second, 8)

	updated, _ := m.Update(snapshotsMsg{
		results: []hostSnapshotResult{
			{
				index: 0,
				snapshot: model.HostSnapshot{
					HostID:       "local",
					Hostname:     "local",
					GPUBackend:   "sample",
					Timestamp:    now,
					CPUUsedPct:   model.NewMetricValue(15),
					GPUs:         []model.GPUSnapshot{{Index: 0, UUID: "GPU-0", GPUUtilPct: model.NewMetricValue(30)}},
					GPUProcesses: []model.GPUProcess{{PID: 11}},
				},
			},
			{
				index: 1,
				err:   errors.New("timeout"),
			},
		},
	})

	modelState := updated.(Model)
	if modelState.hosts[0].snapshot.GPUBackend != "sample" {
		t.Fatalf("gpu backend = %q, want sample", modelState.hosts[0].snapshot.GPUBackend)
	}
	if modelState.hosts[1].lastError == "" {
		t.Fatal("expected remote host error")
	}

	updated, _ = modelState.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	modelState = updated.(Model)
	if modelState.selectedIndex != 0 {
		t.Fatalf("selected index = %d, want 0", modelState.selectedIndex)
	}
}

func TestModelMouseSelectHost(t *testing.T) {
	m := New([]hostSource{
		stubHostSource{name: "local"},
		stubHostSource{name: "remote"},
	}, time.Second, 8)
	m.width = 120
	m.height = 40

	if index, ok := m.hostSelectorHit(2, 3); !ok || index != -1 {
		t.Fatalf("all hit = (%d, %t), want (-1, true)", index, ok)
	}

	updated, _ := m.Update(tea.MouseMsg{
		X:      2,
		Y:      4,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	modelState := updated.(Model)
	if modelState.selectedIndex != 0 {
		t.Fatalf("selected index = %d, want 0 after mouse select", modelState.selectedIndex)
	}
}

func TestWarningsToggle(t *testing.T) {
	m := New([]hostSource{
		stubHostSource{name: "local"},
	}, time.Second, 8)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	modelState := updated.(Model)
	if !modelState.showWarnings {
		t.Fatal("expected warnings dialog to be shown")
	}

	updated, _ = modelState.Update(tea.KeyMsg{Type: tea.KeyEsc})
	modelState = updated.(Model)
	if modelState.showWarnings {
		t.Fatal("expected warnings dialog to be hidden")
	}
}

func TestProcessVizToggle(t *testing.T) {
	m := New([]hostSource{
		stubHostSource{name: "local"},
	}, time.Second, 8)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	modelState := updated.(Model)
	if !modelState.showProcessViz {
		t.Fatal("expected process viz to be enabled")
	}

	updated, _ = modelState.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	modelState = updated.(Model)
	if modelState.showProcessViz {
		t.Fatal("expected process viz to be disabled")
	}
}

func TestProcessViewportScrollKeys(t *testing.T) {
	m := New([]hostSource{
		stubHostSource{name: "local"},
	}, time.Second, 8)
	m.width = 120
	m.height = 12
	m.hosts[0].snapshot = model.HostSnapshot{
		HostID:   "local",
		Hostname: "local",
		GPUs: []model.GPUSnapshot{
			{Index: 0, UUID: "GPU-0"},
		},
		GPUProcesses: []model.GPUProcess{
			{PID: 1, GPUIndex: 0},
			{PID: 2, GPUIndex: 0},
			{PID: 3, GPUIndex: 0},
			{PID: 4, GPUIndex: 0},
			{PID: 5, GPUIndex: 0},
			{PID: 6, GPUIndex: 0},
		},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	modelState := updated.(Model)
	if modelState.processScroll == 0 {
		t.Fatal("expected process viewport to scroll down")
	}

	updated, _ = modelState.Update(tea.KeyMsg{Type: tea.KeyEnd})
	modelState = updated.(Model)
	if modelState.processScroll != modelState.maxProcessScroll() {
		t.Fatalf("process scroll = %d, want max %d", modelState.processScroll, modelState.maxProcessScroll())
	}

	updated, _ = modelState.Update(tea.KeyMsg{Type: tea.KeyHome})
	modelState = updated.(Model)
	if modelState.processScroll != 0 {
		t.Fatalf("process scroll = %d, want 0", modelState.processScroll)
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

func TestNewFromConfigRemoteSourceRoundTrip(t *testing.T) {
	sample := collector.NewSampleCollector("remote-host", "remote-host", time.Second)
	server := httpapi.NewServer(sample, "")

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()
			server.Handler().ServeHTTP(rec, req)
			return recorderResponse(rec), nil
		}),
	}

	source := remoteSource{
		name: "remote-a",
		client: httpapi.Client{
			BaseURL:    "http://nvimon.test",
			HTTPClient: client,
		},
	}

	snapshot, err := source.Fetch(context.Background())
	if err != nil {
		t.Fatalf("fetch remote source: %v", err)
	}
	if snapshot.GPUBackend != "sample" {
		t.Fatalf("gpu backend = %q, want sample", snapshot.GPUBackend)
	}
	if snapshot.HostID != "remote-a" {
		t.Fatalf("host id = %q, want remote-a", snapshot.HostID)
	}
}
