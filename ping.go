package main

import (
	"fmt"
	"net/http"
	"sort"
	"time"
)

type PingResult struct {
	URL        string
	StatusCode int
	Status     string
	Timestamp  time.Time
}

func pingURL(url string) PingResult {
	resp, err := http.Get(url)
	if err != nil {
		return PingResult{
			URL:        url,
			StatusCode: http.StatusInternalServerError,
			Status:     fmt.Sprintf("Failed to ping: %s", err.Error()),
			Timestamp:  time.Now(),
		}
	}
	defer resp.Body.Close()

	return PingResult{
		URL:        url,
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Timestamp:  time.Now(),
	}
}

func formatPingResults(results []PingResult) string {
	// Group results by status code
	statusGroups := make(map[int][]PingResult)
	for _, result := range results {
		statusGroups[result.StatusCode] = append(statusGroups[result.StatusCode], result)
	}

	// Get all status codes and sort them (non-200 first)
	var statusCodes []int
	for code := range statusGroups {
		statusCodes = append(statusCodes, code)
	}
	sort.Slice(statusCodes, func(i, j int) bool {
		if statusCodes[i] == 200 {
			return false
		}
		if statusCodes[j] == 200 {
			return true
		}
		return statusCodes[i] < statusCodes[j]
	})

	var output string
	output += "📊 Статус проверки сайтов:\n\n"

	for _, code := range statusCodes {
		groupResults := statusGroups[code]
		
		// Add emoji based on status code
		emoji := "✅"
		if code != 200 {
			emoji = "⚠️"
		}
		
		output += fmt.Sprintf("%s Статус %d:\n", emoji, code)
		for _, result := range groupResults {
			output += fmt.Sprintf("  • %s\n", result.URL)
		}
		output += "\n"
	}
	return output
}
