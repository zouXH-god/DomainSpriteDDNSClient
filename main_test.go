package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func testClient() *http.Client { return &http.Client{Timeout: 2 * time.Second} }

func TestCreateUsesPOSTAndAccessSalt(t *testing.T) {
	var method, salt string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, salt = r.Method, r.Header.Get("AccessSalt")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"message":"Success","data":{"token":"token-value","recordInfo":{"id":"1","domainName":"example.com","recordName":"host","recordContent":"127.0.0.1"}}}`))
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "state", "data.json")
	if err := makeIP2A(context.Background(), testClient(), server.URL+"/", "secret-salt-value", path); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPost || salt != "secret-salt-value" {
		t.Fatalf("method=%s salt=%q", method, salt)
	}
	saved, err := loadData(path)
	if err != nil || saved.Token != "token-value" {
		t.Fatalf("saved=%+v err=%v", saved, err)
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(path)
		if info.Mode().Perm() != 0600 {
			t.Fatalf("state mode=%o", info.Mode().Perm())
		}
	}
}

func TestUpdateUsesPUTAndKeepsTokenOutOfURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	if err := saveData(path, SavedData{Token: "private-token", RecordInfo: RecordInfo{DomainName: "example.com", RecordName: "host"}}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method=%s", r.Method)
		}
		if r.URL.RawQuery != "" || strings.Contains(r.RequestURI, "private-token") {
			t.Errorf("token leaked in URL: %s", r.RequestURI)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["token"] != "private-token" {
			t.Errorf("body=%v err=%v", body, err)
		}
		_, _ = w.Write([]byte(`{"code":200,"message":"Success","data":{"token":"private-token","recordInfo":{"domainName":"example.com","recordName":"host","recordContent":"127.0.0.2"}}}`))
	}))
	defer server.Close()
	if err := sendRecordUpdate(context.Background(), testClient(), server.URL, path); err != nil {
		t.Fatal(err)
	}
}

func TestLoadLegacyState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	if err := os.WriteFile(path, []byte(`{"data":{"token":"legacy-token","recordInfo":{"domainName":"example.com"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	saved, err := loadData(path)
	if err != nil || saved.Token != "legacy-token" {
		t.Fatalf("saved=%+v err=%v", saved, err)
	}
}

func TestAPIErrorIncludesServerMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"code":403,"message":"AccessSalt is error"}`))
	}))
	defer server.Close()
	err := makeIP2A(context.Background(), testClient(), server.URL, "wrong-value", filepath.Join(t.TempDir(), "data.json"))
	if err == nil || !strings.Contains(err.Error(), "AccessSalt is error") {
		t.Fatalf("err=%v", err)
	}
}
