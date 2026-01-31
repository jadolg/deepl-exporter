package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type DeepLUsage struct {
	CharacterCount int64 `json:"character_count"`
	CharacterLimit int64 `json:"character_limit"`
}

type DeepLCollector struct {
	apiKey            string
	apiURL            string
	characterCount    *prometheus.Desc
	characterLimit    *prometheus.Desc
	characterUsagePct *prometheus.Desc
}

func NewDeepLCollector(apiKey string) *DeepLCollector {
	// Detect API type from key suffix
	// Free API keys end with ":fx"
	// Pro API keys do not have this suffix
	apiURL := "https://api.deepl.com/v2/usage"
	if len(apiKey) > 3 && apiKey[len(apiKey)-3:] == ":fx" {
		apiURL = "https://api-free.deepl.com/v2/usage"
		log.Println("Detected DeepL Free API key")
	} else {
		log.Println("Detected DeepL Pro API key")
	}

	return &DeepLCollector{
		apiKey: apiKey,
		apiURL: apiURL,
		characterCount: prometheus.NewDesc(
			"deepl_character_count",
			"Current number of characters translated in the current billing period",
			nil,
			nil,
		),
		characterLimit: prometheus.NewDesc(
			"deepl_character_limit",
			"Maximum number of characters that can be translated in the current billing period",
			nil,
			nil,
		),
		characterUsagePct: prometheus.NewDesc(
			"deepl_character_usage_percent",
			"Percentage of character limit used",
			nil,
			nil,
		),
	}
}

func (c *DeepLCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.characterCount
	ch <- c.characterLimit
	ch <- c.characterUsagePct
}

func (c *DeepLCollector) Collect(ch chan<- prometheus.Metric) {
	usage, err := c.fetchUsage()
	if err != nil {
		log.Printf("Error fetching DeepL usage: %v", err)
		return
	}

	ch <- prometheus.MustNewConstMetric(
		c.characterCount,
		prometheus.GaugeValue,
		float64(usage.CharacterCount),
	)

	ch <- prometheus.MustNewConstMetric(
		c.characterLimit,
		prometheus.GaugeValue,
		float64(usage.CharacterLimit),
	)

	usagePercent := 0.0
	if usage.CharacterLimit > 0 {
		usagePercent = (float64(usage.CharacterCount) / float64(usage.CharacterLimit)) * 100
	}

	ch <- prometheus.MustNewConstMetric(
		c.characterUsagePct,
		prometheus.GaugeValue,
		usagePercent,
	)
}

func (c *DeepLCollector) fetchUsage() (*DeepLUsage, error) {
	req, err := http.NewRequest("GET", c.apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("DeepL-Auth-Key %s", c.apiKey))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch usage: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var usage DeepLUsage
	if err := json.Unmarshal(body, &usage); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &usage, nil
}

func main() {
	apiKey := os.Getenv("DEEPL_API_KEY")
	if apiKey == "" {
		log.Fatal("DEEPL_API_KEY environment variable is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "1818"
	}

	collector := NewDeepLCollector(apiKey)
	prometheus.MustRegister(collector)

	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	log.Printf("Starting DeepL Prometheus exporter on port %s", port)
	log.Printf("Metrics available at http://localhost:%s/metrics", port)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
