package documents

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOWUIUploadFile(t *testing.T) {
	var gotMethod, gotPath, gotAuth, gotFieldName, gotFilename, gotContent string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")

		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		f, hdr, err := r.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		gotFieldName = "file"
		gotFilename = hdr.Filename
		b, _ := io.ReadAll(f)
		gotContent = string(b)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"file-123"}`))
	}))
	defer srv.Close()

	c := &OWUIClient{BaseURL: srv.URL, APIKey: "secret-key"}
	id, err := c.UploadFile(context.Background(), "notlar.txt", []byte("merhaba"))
	if err != nil {
		t.Fatal(err)
	}
	if id != "file-123" {
		t.Fatalf("beklenmedik id: %q", id)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("beklenmedik method: %q", gotMethod)
	}
	if gotPath != "/api/v1/files" {
		t.Fatalf("beklenmedik path: %q", gotPath)
	}
	if gotAuth != "Bearer secret-key" {
		t.Fatalf("beklenmedik auth: %q", gotAuth)
	}
	if gotFieldName != "file" || gotFilename != "notlar.txt" || gotContent != "merhaba" {
		t.Fatalf("beklenmedik form: field=%q filename=%q content=%q", gotFieldName, gotFilename, gotContent)
	}
}

func TestOWUIUploadFileWrappedID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"file":{"id":"wrapped-1"}}`))
	}))
	defer srv.Close()

	c := &OWUIClient{BaseURL: srv.URL}
	id, err := c.UploadFile(context.Background(), "a.txt", []byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	if id != "wrapped-1" {
		t.Fatalf("beklenmedik id: %q", id)
	}
}

func TestOWUIAddFileToKnowledge(t *testing.T) {
	var gotMethod, gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &OWUIClient{BaseURL: srv.URL, APIKey: "k"}
	if err := c.AddFileToKnowledge(context.Background(), "kb-1", "file-9"); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/v1/knowledge/kb-1/file/add" {
		t.Fatalf("beklenmedik istek: %s %s", gotMethod, gotPath)
	}
	if !strings.Contains(gotBody, `"file_id":"file-9"`) {
		t.Fatalf("beklenmedik govde: %q", gotBody)
	}
}

func TestOWUIRemoveFileFromKnowledge(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &OWUIClient{BaseURL: srv.URL}
	if err := c.RemoveFileFromKnowledge(context.Background(), "kb-1", "file-9"); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/api/v1/knowledge/kb-1/file/remove" {
		t.Fatalf("beklenmedik path: %q", gotPath)
	}
}

func TestOWUIDeleteFile(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &OWUIClient{BaseURL: srv.URL}
	if err := c.DeleteFile(context.Background(), "file-9"); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/api/v1/files/file-9" {
		t.Fatalf("beklenmedik istek: %s %s", gotMethod, gotPath)
	}
}

func TestOWUIErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("nope"))
	}))
	defer srv.Close()

	c := &OWUIClient{BaseURL: srv.URL}
	if err := c.DeleteFile(context.Background(), "x"); err == nil {
		t.Fatal("hata bekleniyordu")
	}
}
