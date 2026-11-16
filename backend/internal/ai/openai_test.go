package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestChatCompletionsURL(t *testing.T) {
	cases := []struct{ base, want string }{
		{"https://api.openai.com/v1", "https://api.openai.com/v1/chat/completions"},
		{"https://api.openai.com/v1/", "https://api.openai.com/v1/chat/completions"},
		{"https://api.openai.com/v1/chat/completions", "https://api.openai.com/v1/chat/completions"},
		{"https://relay.example.com/api/v1/chat/completions/", "https://relay.example.com/api/v1/chat/completions"},
		{"https://gateway.example.com", "https://gateway.example.com/v1/chat/completions"},
		{"https://gateway.example.com/openai", "https://gateway.example.com/openai/v1/chat/completions"},
		{"", defaultBaseURL + "/chat/completions"},
	}
	for _, tc := range cases {
		if got := chatCompletionsURL(tc.base); got != tc.want {
			t.Errorf("chatCompletionsURL(%q)=%q want %q", tc.base, got, tc.want)
		}
	}
}

func okCompletionBody(content string) string {
	b, _ := json.Marshal(map[string]any{
		"choices": []map[string]any{{"message": map[string]string{"content": content}}},
	})
	return string(b)
}

func TestOpenAIProviderRequestAndSuccess(t *testing.T) {
	for _, jsonFormat := range []bool{false, true} {
		var gotPath, gotAuth, gotCT string
		var gotBody map[string]any
		var calls int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&calls, 1)
			gotPath = r.URL.Path
			gotAuth = r.Header.Get("Authorization")
			gotCT = r.Header.Get("Content-Type")
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(okCompletionBody(`{"domain":"ai","score":80,"keep":true}`)))
		}))
		defer srv.Close()

		p := NewOpenAIProvider(srv.URL+"/v1", "sk-test-key", "test-model", jsonFormat, 3*time.Second)
		if p.Name() != "openai" {
			t.Errorf("name: %q", p.Name())
		}
		out, err := p.Complete(context.Background(), "sys", "user")
		if err != nil {
			t.Fatalf("Complete: %v", err)
		}
		if !strings.Contains(out, `"score":80`) {
			t.Errorf("unexpected completion: %q", out)
		}
		if atomic.LoadInt32(&calls) != 1 {
			t.Errorf("expected 1 call, got %d", calls)
		}
		if gotPath != "/v1/chat/completions" {
			t.Errorf("path: %q", gotPath)
		}
		if gotAuth != "Bearer sk-test-key" {
			t.Errorf("auth header: %q", gotAuth)
		}
		if gotCT != "application/json" {
			t.Errorf("content-type: %q", gotCT)
		}
		if gotBody["model"] != "test-model" {
			t.Errorf("model: %v", gotBody["model"])
		}
		msgs, _ := gotBody["messages"].([]any)
		if len(msgs) != 2 {
			t.Fatalf("messages: %v", gotBody["messages"])
		}
		_, hasFormat := gotBody["response_format"]
		if hasFormat != jsonFormat {
			t.Errorf("response_format presence=%v want %v", hasFormat, jsonFormat)
		}
	}
}

func TestOpenAIProviderErrorStatuses(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusUnauthorized} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":"upstream problem"}`))
		}))
		p := NewOpenAIProvider(srv.URL, "sk-test-key", "m", false, 2*time.Second)
		_, err := p.Complete(context.Background(), "sys", "user")
		if err == nil {
			t.Errorf("status %d should error", status)
		} else if !strings.Contains(err.Error(), "status") {
			t.Errorf("error should mention status: %v", err)
		}
		srv.Close()
	}
}

func TestOpenAIProviderMalformedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()
	p := NewOpenAIProvider(srv.URL, "k", "m", false, 2*time.Second)
	if _, err := p.Complete(context.Background(), "s", "u"); err == nil {
		t.Errorf("malformed response should error")
	}
}

func TestOpenAIProviderEmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer srv.Close()
	p := NewOpenAIProvider(srv.URL, "k", "m", false, 2*time.Second)
	if _, err := p.Complete(context.Background(), "s", "u"); err == nil {
		t.Errorf("empty choices should error")
	}
}

func TestOpenAIProviderTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		_, _ = w.Write([]byte(okCompletionBody("{}")))
	}))
	defer srv.Close()
	p := NewOpenAIProvider(srv.URL, "k", "m", false, 50*time.Millisecond)
	start := time.Now()
	_, err := p.Complete(context.Background(), "s", "u")
	if err == nil {
		t.Fatalf("expected timeout error")
	}
	if elapsed := time.Since(start); elapsed > 250*time.Millisecond {
		t.Errorf("timeout not enforced quickly: %v", elapsed)
	}
}

func TestOpenAIProviderRedactsKeyInError(t *testing.T) {
	const key = "sk-super-secret-key-999"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"bad key ` + key + `"}`))
	}))
	defer srv.Close()
	p := NewOpenAIProvider(srv.URL, key, "m", false, 2*time.Second)
	_, err := p.Complete(context.Background(), "s", "u")
	if err == nil {
		t.Fatalf("expected error")
	}
	if strings.Contains(err.Error(), key) {
		t.Errorf("API key leaked in error: %v", err)
	}
	if !strings.Contains(err.Error(), "[REDACTED]") {
		t.Errorf("expected redaction marker: %v", err)
	}
}

func TestOpenAIProviderContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte(okCompletionBody("{}")))
	}))
	defer srv.Close()
	p := NewOpenAIProvider(srv.URL, "k", "m", false, 2*time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Complete(ctx, "s", "u"); err == nil {
		t.Errorf("cancelled context should error")
	}
}