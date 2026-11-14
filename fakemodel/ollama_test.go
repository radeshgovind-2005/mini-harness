package fakemodel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A loopback httptest server stands in for Ollama: no real model, no outside network.
func TestOllamaMapsTextAndToolCalls(t *testing.T) {
	var gotModel, gotLast string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model    string `json:"model"`
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		gotModel, gotLast = req.Model, req.Messages[len(req.Messages)-1].Content
		w.Write([]byte(`{"message":{"role":"assistant","content":"hi","tool_calls":[{"function":{"name":"send","arguments":{"to":"a@b.c","n":2}}}]}}`))
	}))
	defer srv.Close()

	var m Model = &Ollama{BaseURL: srv.URL, ModelID: "tiny"}
	r, err := m.Complete(context.Background(), []Message{{Role: "user", Content: "ping"}})
	if err != nil {
		t.Fatal(err)
	}
	if gotModel != "tiny" || gotLast != "ping" {
		t.Fatalf("request model=%q last=%q", gotModel, gotLast)
	}
	if r.Text != "hi" || len(r.ToolCalls) != 1 || r.ToolCalls[0].Name != "send" ||
		r.ToolCalls[0].Args["to"] != "a@b.c" || r.ToolCalls[0].Args["n"] != "2" {
		t.Fatalf("response = %+v", r)
	}
}

func TestOllamaErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	if _, err := (&Ollama{BaseURL: srv.URL, ModelID: "x"}).Complete(context.Background(), nil); err == nil {
		t.Fatal("want error on 500")
	}
}
