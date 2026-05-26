package storage

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"camera/internal/db"
)

func TestS3Drive_Upload(t *testing.T) {
	var receivedKey string
	var receivedAuth string
	var receivedBody string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedKey = r.URL.Path
		receivedAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	drive := NewS3Drive(db.Drive{
		Name:      "test",
		Type:      "s3",
		Endpoint:  srv.URL,
		Bucket:    "my-bucket",
		Region:    "us-east-1",
		AccessKey: "AKID",
		SecretKey: "secret",
	})

	content := "hello s3"
	err := drive.Upload(t.Context(), "recordings/test.mp4", strings.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	if receivedBody != content {
		t.Errorf("body = %q, want %q", receivedBody, content)
	}
	if !strings.Contains(receivedAuth, "AWS4-HMAC-SHA256") {
		t.Errorf("auth header missing AWS4-HMAC-SHA256: %s", receivedAuth)
	}
	if !strings.Contains(receivedKey, "test.mp4") {
		t.Errorf("key path missing test.mp4: %s", receivedKey)
	}
}

func TestS3Drive_UploadWithPrefix(t *testing.T) {
	var receivedKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedKey = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	drive := NewS3Drive(db.Drive{
		Type:      "s3",
		Endpoint:  srv.URL,
		Bucket:    "bucket",
		Region:    "eu-west-1",
		AccessKey: "AK",
		SecretKey: "SK",
		Prefix:    "archive",
	})

	_ = drive.Upload(t.Context(), "cam1/2025/01/15/file.mp4", strings.NewReader("x"), 1)
	if !strings.Contains(receivedKey, "archive") {
		t.Errorf("prefix not in key: %s", receivedKey)
	}
}

func TestS3Drive_PlusInPrefixEncodedAsPct2B(t *testing.T) {
	var rawPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawPath = r.URL.RawPath
		if rawPath == "" {
			rawPath = r.URL.Path
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	drive := NewS3Drive(db.Drive{
		Type:      "s3",
		Endpoint:  srv.URL,
		Bucket:    "bucket",
		Region:    "us-east-1",
		AccessKey: "AK",
		SecretKey: "SK",
		Prefix:    "my+prefix/sub",
	})

	_ = drive.Upload(t.Context(), "cam/file.mp4", strings.NewReader("x"), 1)

	if strings.Contains(rawPath, "+") {
		t.Errorf("'+' must be encoded as %%2B in URL path, got: %s", rawPath)
	}
	if !strings.Contains(rawPath, "%2B") {
		t.Errorf("'+' not encoded as %%2B in URL path: %s", rawPath)
	}
	if strings.Contains(rawPath, "%2F") {
		t.Errorf("'/' must not be encoded as %%2F in URL path: %s", rawPath)
	}
}

func TestSlugify(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Corredor da Frente", "corredor-da-frente"},
		{"corredor-da-frente", "corredor-da-frente"},
		{"Câmera 01", "c-mera-01"},
		{"  spaces  ", "spaces"},
		{"cam/1", "cam-1"},
	}
	for _, tc := range cases {
		got := slugify(tc.in)
		if got != tc.want {
			t.Errorf("slugify(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestS3Drive_PlusInPrefixEncodedAsPct2B(t *testing.T) {
	var rawPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawPath = r.URL.RawPath
		if rawPath == "" {
			rawPath = r.URL.Path
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	drive := NewS3Drive(db.Drive{
		Type:      "s3",
		Endpoint:  srv.URL,
		Bucket:    "bucket",
		Region:    "us-east-1",
		AccessKey: "AK",
		SecretKey: "SK",
		Prefix:    "my+prefix/sub",
	})

	_ = drive.Upload(t.Context(), "cam/file.mp4", strings.NewReader("x"), 1)

	if strings.Contains(rawPath, "+") {
		t.Errorf("'+' must be encoded as %%2B in URL path, got: %s", rawPath)
	}
	if !strings.Contains(rawPath, "%2B") {
		t.Errorf("'+' not encoded as %%2B in URL path: %s", rawPath)
	}
	if strings.Contains(rawPath, "%2F") {
		t.Errorf("'/' must not be encoded as %%2F in URL path: %s", rawPath)
	}
}

func TestS3Drive_BucketInCanonicalURI(t *testing.T) {
	var receivedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	drive := NewS3Drive(db.Drive{
		Type:      "s3",
		Endpoint:  srv.URL,
		Bucket:    "my-bucket",
		Region:    "us-east-1",
		AccessKey: "AK",
		SecretKey: "SK",
	})

	_ = drive.Upload(t.Context(), "cam/file.mp4", strings.NewReader("x"), 1)

	if !strings.HasPrefix(receivedPath, "/my-bucket/") {
		t.Errorf("bucket must appear as first path segment, got: %s", receivedPath)
	}
}

func TestHostFromURL(t *testing.T) {
	cases := []struct {
		endpoint string
		want     string
	}{
		{"https://s3.us-east-1.amazonaws.com", "s3.us-east-1.amazonaws.com"},
		{"http://localhost:9000", "localhost:9000"},
		{"https://minio.example.com", "minio.example.com"},
	}
	for _, tc := range cases {
		got := hostFromURL(tc.endpoint)
		if got != tc.want {
			t.Errorf("hostFromURL(%q) = %q, want %q", tc.endpoint, got, tc.want)
		}
	}
}

func TestS3Drive_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "NoSuchBucket", http.StatusNotFound)
	}))
	defer srv.Close()

	drive := NewS3Drive(db.Drive{
		Type:      "s3",
		Endpoint:  srv.URL,
		Bucket:    "bucket",
		Region:    "us-east-1",
		AccessKey: "AK",
		SecretKey: "SK",
	})
	err := drive.Upload(t.Context(), "file.mp4", strings.NewReader("x"), 1)
	if err == nil {
		t.Fatal("expected error on 404, got nil")
	}
}
