package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/actions/scaleset"
)

func TestLabelsMatch(t *testing.T) {
	label := func(names ...string) []scaleset.Label {
		var labels []scaleset.Label
		for _, n := range names {
			labels = append(labels, scaleset.Label{Name: n, Type: "User"})
		}
		return labels
	}

	tests := []struct {
		name     string
		existing []scaleset.Label
		desired  []scaleset.Label
		want     bool
	}{
		{"identical", label("linux", "x64"), label("linux", "x64"), true},
		{"same set different order", label("x64", "linux"), label("linux", "x64"), true},
		{"different labels", label("linux"), label("windows"), false},
		{"extra label", label("linux", "x64"), label("linux"), false},
		{"missing label", label("linux"), label("linux", "x64"), false},
		{"both empty", nil, nil, true},
		{"existing empty", nil, label("linux"), false},
		{"desired empty", label("linux"), nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := labelsMatch(tt.existing, tt.desired)
			if got != tt.want {
				t.Errorf("labelsMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetOrCreateScaleSetUsesRunnerGroupID(t *testing.T) {
	const runnerGroupID = 37
	var getRequestGroupID string
	var createRequestGroupID int
	var createRequestName string
	server := newScaleSetTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			getRequestGroupID = r.URL.Query().Get("runnerGroupId")
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"count":0,"value":[]}`)
		case http.MethodPost:
			var request scaleset.RunnerScaleSet
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode create request: %v", err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			createRequestGroupID = request.RunnerGroupID
			createRequestName = request.Name
			request.ID = 9
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(request); err != nil {
				t.Errorf("encode create response: %v", err)
			}
		default:
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
		}
	}))

	client, err := scaleset.NewClientWithPersonalAccessToken(scaleset.NewClientWithPersonalAccessTokenConfig{
		GitHubConfigURL:     server.URL + "/org",
		PersonalAccessToken: "token",
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := getOrCreateScaleSet(context.Background(), slog.Default(), client, runnerGroupID, "linux", []scaleset.Label{{Name: "self-hosted", Type: "User"}})
	if err != nil {
		t.Fatalf("getOrCreateScaleSet: %v", err)
	}
	if getRequestGroupID != fmt.Sprint(runnerGroupID) {
		t.Errorf("lookup runnerGroupId = %q, want %d", getRequestGroupID, runnerGroupID)
	}
	if createRequestGroupID != runnerGroupID {
		t.Errorf("create runnerGroupId = %d, want %d", createRequestGroupID, runnerGroupID)
	}
	if createRequestName != "linux" || got.ID != 9 {
		t.Errorf("created scale set = %+v, request name = %q", got, createRequestName)
	}
}

func newScaleSetTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/actions/runner-registration"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"url":%q,"token":"eyJhbGciOiJub25lIn0.eyJleHAiOjQxMDI0NDQ4MDB9."}`, server.URL+"/tenant/123/")
		case strings.HasSuffix(r.URL.Path, "/actions/runners/registration-token"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprint(w, `{"token":"registration-token"}`)
		default:
			handler.ServeHTTP(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}
