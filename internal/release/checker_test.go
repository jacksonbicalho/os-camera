package release

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const sampleManifest = `{
  "latest": "v1.4.0-dev",
  "notes_md": "### Novidades\n- algo",
  "min_supported": "v0.0.0",
  "image": "jacksonbicalho/os-camera:1.4.0-dev",
  "assets": {
    "linux-amd64": { "name": "camera-linux-amd64", "sha256": "abc" }
  }
}`

func TestCheckerCheckSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(sampleManifest))
	}))
	defer srv.Close()

	c := NewChecker(srv.URL, "v1.3.0-dev", srv.Client())
	m, err := c.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if m.Latest != "v1.4.0-dev" {
		t.Errorf("Latest = %q, quero v1.4.0-dev", m.Latest)
	}

	st := c.Status()
	if st.Latest != "v1.4.0-dev" || st.Image != "jacksonbicalho/os-camera:1.4.0-dev" {
		t.Errorf("Status = %+v", st)
	}
	if st.Current != "v1.3.0-dev" {
		t.Errorf("Current = %q, quero v1.3.0-dev", st.Current)
	}
	if !st.UpdateAvailable {
		t.Error("UpdateAvailable deveria ser true (v1.4 > v1.3)")
	}
	if st.CheckedAt.IsZero() {
		t.Error("CheckedAt não deveria ser zero após Check")
	}
	if st.Err != "" {
		t.Errorf("Err = %q, quero vazio", st.Err)
	}
}

func TestCheckerCheckErrors(t *testing.T) {
	t.Run("404", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "nope", http.StatusNotFound)
		}))
		defer srv.Close()
		c := NewChecker(srv.URL, "v1.3.0-dev", srv.Client())
		if _, err := c.Check(context.Background()); err == nil {
			t.Error("esperava erro em 404")
		}
		if c.Status().Err == "" {
			t.Error("Err deveria ficar no cache após falha")
		}
	})

	t.Run("json inválido", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("{not json"))
		}))
		defer srv.Close()
		c := NewChecker(srv.URL, "v1.3.0-dev", srv.Client())
		if _, err := c.Check(context.Background()); err == nil {
			t.Error("esperava erro em JSON inválido")
		}
	})
}

func TestUpdateAvailable(t *testing.T) {
	cases := []struct {
		name             string
		current, latest  string
		want             bool
	}{
		{"latest maior", "v1.3.0-dev", "v1.4.0-dev", true},
		{"igual", "v1.4.0-dev", "v1.4.0-dev", false},
		{"latest menor", "v1.4.0-dev", "v1.3.0-dev", false},
		{"current inválido", "dev", "v1.4.0-dev", false},
		{"latest inválido", "v1.3.0-dev", "", false},
		{"ambos inválidos", "dev", "snapshot", false},
		{"patch maior", "v1.4.0-dev", "v1.4.1-dev", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := updateAvailable(tc.current, tc.latest); got != tc.want {
				t.Errorf("updateAvailable(%q, %q) = %v, quero %v", tc.current, tc.latest, got, tc.want)
			}
		})
	}
}

func TestStatusWithoutCheck(t *testing.T) {
	c := NewChecker("http://example.invalid", "v1.3.0-dev", nil)
	st := c.Status()
	if st.Current != "v1.3.0-dev" {
		t.Errorf("Current = %q", st.Current)
	}
	if st.Latest != "" || st.UpdateAvailable {
		t.Errorf("antes do 1º check: Latest deveria estar vazio e UpdateAvailable false: %+v", st)
	}
}
