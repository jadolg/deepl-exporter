package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	defaultTimeout = 10 * time.Second
	proAPIURL      = "https://api.deepl.com/v2/usage"
	freeAPIURL     = "https://api-free.deepl.com/v2/usage"
)

type DeepLUsage struct {
	CharacterCount int64 `json:"character_count"`
	CharacterLimit int64 `json:"character_limit"`
}

type DeepLCollector struct {
	apiKey            string
	apiURL            string
	client            *http.Client
	characterCount    *prometheus.Desc
	characterLimit    *prometheus.Desc
	characterUsagePct *prometheus.Desc
}

func NewDeepLCollector(apiKey string) *DeepLCollector {
	apiURL := proAPIURL
	if len(apiKey) > 3 && apiKey[len(apiKey)-3:] == ":fx" {
		apiURL = freeAPIURL
		log.Println("Detected DeepL Free API key")
	} else {
		log.Println("Detected DeepL Pro API key")
	}

	return &DeepLCollector{
		apiKey: apiKey,
		apiURL: apiURL,
		client: &http.Client{
			Timeout: defaultTimeout,
		},
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
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	usage, err := c.fetchUsage(ctx)
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

func (c *DeepLCollector) fetchUsage(ctx context.Context) (*DeepLUsage, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("DeepL-Auth-Key %s", c.apiKey))

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch usage: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()

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
