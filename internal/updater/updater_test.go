package updater

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNeedsUpdate(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{
			name:    "misma version sin prefijo v",
			current: "0.3.0",
			latest:  "v0.3.0",
			want:    false,
		},
		{
			name:    "misma version con prefijo v",
			current: "v0.3.0",
			latest:  "v0.3.0",
			want:    false,
		},
		{
			name:    "version nueva disponible",
			current: "v0.2.0",
			latest:  "v0.3.0",
			want:    true,
		},
		{
			name:    "version actual mayor (downgrade)",
			current: "v0.4.0",
			latest:  "v0.3.0",
			want:    true,
		},
		{
			name:    "build de desarrollo siempre necesita update",
			current: "dev",
			latest:  "v0.3.0",
			want:    true,
		},
		{
			name:    "version vacia siempre necesita update",
			current: "",
			latest:  "v0.3.0",
			want:    true,
		},
		{
			name:    "versiones identicas con v ambas",
			current: "v1.0.0",
			latest:  "v1.0.0",
			want:    false,
		},
		{
			name:    "patch diferente",
			current: "v0.3.0",
			latest:  "v0.3.1",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NeedsUpdate(tt.current, tt.latest)
			if got != tt.want {
				t.Errorf("NeedsUpdate(%q, %q) = %v, queria %v", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestCheckLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/wirvii/agent-speech/releases/latest" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		resp := releaseResponse{
			TagName: "v0.5.0",
			Assets: []releaseAsset{
				{Name: "agent-speech-linux-amd64", BrowserDownloadURL: "https://example.com/agent-speech-linux-amd64"},
				{Name: "agent-speech-darwin-arm64", BrowserDownloadURL: "https://example.com/agent-speech-darwin-arm64"},
			},
		}
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	defer srv.Close()

	// Parchear la base URL para apuntar al servidor de prueba
	original := githubAPIBase
	githubAPIBase = srv.URL
	defer func() { githubAPIBase = original }()

	tag, err := CheckLatest("wirvii/agent-speech")
	if err != nil {
		t.Fatalf("CheckLatest error inesperado: %v", err)
	}
	if tag != "v0.5.0" {
		t.Errorf("CheckLatest retorno %q, queria %q", tag, "v0.5.0")
	}
}

func TestCheckLatest_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	original := githubAPIBase
	githubAPIBase = srv.URL
	defer func() { githubAPIBase = original }()

	_, err := CheckLatest("wirvii/agent-speech")
	if err == nil {
		t.Fatal("se esperaba error pero no hubo")
	}
}

func TestCheckLatest_EmptyTagName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Respuesta con tag_name vacio
		json.NewEncoder(w).Encode(releaseResponse{TagName: ""}) //nolint:errcheck
	}))
	defer srv.Close()

	original := githubAPIBase
	githubAPIBase = srv.URL
	defer func() { githubAPIBase = original }()

	_, err := CheckLatest("wirvii/agent-speech")
	if err == nil {
		t.Fatal("se esperaba error por tag_name vacio pero no hubo")
	}
}

func TestResolveDownloadURL(t *testing.T) {
	const wantURL = "https://example.com/binaries/agent-speech-linux-amd64"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/wirvii/agent-speech/releases/tags/v0.5.0" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		resp := releaseResponse{
			TagName: "v0.5.0",
			Assets: []releaseAsset{
				{Name: "agent-speech-linux-amd64", BrowserDownloadURL: wantURL},
				{Name: "agent-speech-darwin-arm64", BrowserDownloadURL: "https://example.com/binaries/agent-speech-darwin-arm64"},
			},
		}
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	defer srv.Close()

	original := githubAPIBase
	githubAPIBase = srv.URL
	defer func() { githubAPIBase = original }()

	got, err := resolveDownloadURL("wirvii/agent-speech", "v0.5.0", "agent-speech-linux-amd64")
	if err != nil {
		t.Fatalf("resolveDownloadURL error inesperado: %v", err)
	}
	if got != wantURL {
		t.Errorf("resolveDownloadURL retorno %q, queria %q", got, wantURL)
	}
}

func TestResolveDownloadURL_AssetNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := releaseResponse{
			TagName: "v0.5.0",
			Assets:  []releaseAsset{},
		}
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	defer srv.Close()

	original := githubAPIBase
	githubAPIBase = srv.URL
	defer func() { githubAPIBase = original }()

	_, err := resolveDownloadURL("wirvii/agent-speech", "v0.5.0", "agent-speech-windows-amd64")
	if err == nil {
		t.Fatal("se esperaba error por asset no encontrado pero no hubo")
	}
}
