package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestNewDeepLCollector(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   string
		expected string
	}{
		{
			name:     "Free API Key",
			apiKey:   "test-key:fx",
			expected: "https://api-free.deepl.com/v2/usage",
		},
		{
			name:     "Pro API Key",
			apiKey:   "test-key",
			expected: "https://api.deepl.com/v2/usage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewDeepLCollector(tt.apiKey)
			if c.apiURL != tt.expected {
				t.Errorf("expected URL %s, got %s", tt.expected, c.apiURL)
			}
		})
	}
}

func TestDeepLCollector_Collect(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "DeepL-Auth-Key test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintln(w, `{"character_count": 1000, "character_limit": 500000}`)
	}))
	defer ts.Close()

	c := NewDeepLCollector("test-key")
	c.apiURL = ts.URL

	ch := make(chan prometheus.Metric)
	go func() {
		c.Collect(ch)
		close(ch)
	}()

	metrics := make(map[string]float64)
	for m := range ch {
		_ = m
		metrics["count"]++
	}

	if metrics["count"] != 3 {
		t.Errorf("expected 3 metrics, got %v", metrics["count"])
	}
}

func TestDeepLCollector_fetchUsage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "DeepL-Auth-Key test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = fmt.Fprintln(w, `{"character_count": 12345, "character_limit": 500000}`)
	}))
	defer ts.Close()

	c := NewDeepLCollector("test-key")
	c.apiURL = ts.URL

	usage, err := c.fetchUsage(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if usage.CharacterCount != 12345 {
		t.Errorf("expected count 12345, got %d", usage.CharacterCount)
	}
	if usage.CharacterLimit != 500000 {
		t.Errorf("expected limit 500000, got %d", usage.CharacterLimit)
	}
}

func TestDeepLCollector_fetchUsage_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintln(w, "internal error")
	}))
	defer ts.Close()

	c := NewDeepLCollector("test-key")
	c.apiURL = ts.URL

	_, err := c.fetchUsage(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "API returned status 500") {
		t.Errorf("expected status 500 error, got %v", err)
	}
}
