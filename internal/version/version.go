package version

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// VersionInfo holds the version information from npm registry
type VersionInfo struct {
	Version     string
	LastUpdated time.Time
}

var (
	versionInfo *VersionInfo
	versionMu   sync.RWMutex
)

const versionCacheDuration = 30 * time.Minute

// GetCommandCodeVersion fetches the latest version from npm registry
// It caches the result for 30 minutes to avoid excessive API calls
func GetCommandCodeVersion() string {
	// Fast path: check with read lock
	versionMu.RLock()
	info := versionInfo
	versionMu.RUnlock()

	if info != nil && time.Since(info.LastUpdated) < versionCacheDuration {
		return info.Version
	}

	// Slow path: acquire write lock and double-check
	versionMu.Lock()
	defer versionMu.Unlock()

	if versionInfo != nil && time.Since(versionInfo.LastUpdated) < versionCacheDuration {
		return versionInfo.Version
	}

	newInfo, err := fetchVersionFromNPM()
	if err != nil {
		if versionInfo != nil {
			return versionInfo.Version
		}
		return "unknown"
	}

	versionInfo = newInfo
	return newInfo.Version
}

// fetchVersionFromNPM fetches the latest version from npm registry
func fetchVersionFromNPM() (*VersionInfo, error) {
	resp, err := http.Get("https://registry.npmjs.org/command-code/latest")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch version: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("npm registry returned %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse version: %w", err)
	}

	version, ok := result["version"].(string)
	if !ok {
		return nil, fmt.Errorf("version field not found or invalid")
	}

	return &VersionInfo{
		Version:     version,
		LastUpdated: time.Now(),
	}, nil
}

// init fetches the version on startup (in background) and caches it
func init() {
	go func() {
		info, err := fetchVersionFromNPM()
		if err != nil {
			return
		}
		versionMu.Lock()
		versionInfo = info
		versionMu.Unlock()
	}()
}
