# DeepL Prometheus Exporter

A Prometheus exporter for DeepL API usage metrics written in Go.

## Metrics Exposed

- `deepl_character_count` - Current number of characters translated in the billing period
- `deepl_character_limit` - Maximum number of characters available in the billing period
- `deepl_character_usage_percent` - Percentage of character limit used

## Usage

### Run the exporter:

`docker run -e DEEPL_API_KEY=your-api-key -p 1818:1818 ghcr.io/jadolg/deepl-exporter`

## Prometheus Configuration

Add this to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'deepl'
    static_configs:
      - targets: ['localhost:1818']
    scrape_interval: 5m  # Recommended: 5 minutes
```
