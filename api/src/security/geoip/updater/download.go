// Package updater is the write side of plan/geoip-enrichment/plan.md:
// a long-lived process that periodically downloads MaxMind GeoLite2
// CSV data and imports it into a fresh geoip.db, atomically replacing
// the previous one. Runs as its own executable
// (api/cmd/geoipupdater), never as a goroutine inside the main
// coco-iam server and never via cron — see Updater.Run.
package updater

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// maxmindDownloadBaseURL is MaxMind's documented download endpoint
// (https://dev.maxmind.com/geoip/updating-databases#directly-downloading-databases).
// Kept as a var, not a const, so tests can point a Downloader at a
// local httptest.Server instead of the real MaxMind service.
var maxmindDownloadBaseURL = "https://download.maxmind.com/geoip/databases"

// Edition IDs for MaxMind's free-tier CSV-format databases — the
// account-based free GeoLite2 tier, not the paid GeoIP2 editions.
// GeoLite2-City-CSV is a strict superset of GeoLite2-Country-CSV
// (country + city + subdivision + postal code + lat/long, all from
// one pull), so it's used in place of the Country edition rather than
// alongside it — see plan/geoip-enrichment/plan.md's "Extension:
// city-level GeoIP" section.
const (
	editionCityCSV = "GeoLite2-City-CSV"
	editionASNCSV  = "GeoLite2-ASN-CSV"
)

// Downloader fetches a MaxMind GeoLite2 CSV-format database edition,
// authenticated via HTTP Basic auth with a free-tier account ID and
// license key.
type Downloader struct {
	baseURL    string
	accountID  string
	licenseKey string
	httpClient *http.Client
}

// NewDownloader builds a Downloader against the real MaxMind service.
func NewDownloader(accountID, licenseKey string) *Downloader {
	return &Downloader{
		baseURL:    maxmindDownloadBaseURL,
		accountID:  accountID,
		licenseKey: licenseKey,
		httpClient: &http.Client{Timeout: 5 * time.Minute},
	}
}

// Download fetches edition (editionCityCSV or editionASNCSV) as a
// zip archive and writes it to destPath. Every step here is purely
// preparatory — nothing about a download failure ever touches the
// live geoip.db, since destPath is always a temp-directory path the
// caller controls.
func (d *Downloader) Download(ctx context.Context, edition, destPath string) error {
	url := fmt.Sprintf("%s/%s/download?suffix=zip", d.baseURL, edition)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("geoip-updater: build request for %s: %w", edition, err)
	}
	req.SetBasicAuth(d.accountID, d.licenseKey)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("geoip-updater: download %s: %w", edition, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("geoip-updater: download %s: unexpected status %d", edition, resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("geoip-updater: create %s: %w", destPath, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("geoip-updater: write %s: %w", destPath, err)
	}
	return nil
}
