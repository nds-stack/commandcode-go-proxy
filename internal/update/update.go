package update

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const tagsURL = "https://api.github.com/repos/nds-stack/commandcode-go-proxy/tags"

type tagResponse struct {
	Name string `json:"name"`
}

func LatestVersion(current string) (string, bool, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(tagsURL)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("github tags returned %d", resp.StatusCode)
	}

	var tags []tagResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return "", false, err
	}

	latest := latestTag(tags)
	if latest == "" {
		return "", false, nil
	}

	return latest, compareVersion(latest, current) > 0, nil
}

func latestTag(tags []tagResponse) string {
	latest := ""
	for _, tag := range tags {
		if tag.Name == "" {
			continue
		}
		if latest == "" || compareVersion(tag.Name, latest) > 0 {
			latest = tag.Name
		}
	}
	return latest
}

func compareVersion(a, b string) int {
	av := parseVersion(a)
	bv := parseVersion(b)
	for i := 0; i < 3; i++ {
		if av[i] > bv[i] {
			return 1
		}
		if av[i] < bv[i] {
			return -1
		}
	}
	return 0
}

func parseVersion(v string) [3]int {
	v = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(v)), "v")
	parts := strings.Split(v, ".")
	var result [3]int
	for i := 0; i < len(parts) && i < 3; i++ {
		n, _ := strconv.Atoi(parts[i])
		result[i] = n
	}
	return result
}
