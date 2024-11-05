package main

import (
	"fmt"
	"net/http"
	"time"
)

func pingURL(url string) (string, int) {
	client := http.Client{
		Timeout: 20 * time.Second,
	}
	resp, err := client.Get(url)

	if err != nil {
		return fmt.Sprintf("❌ %s is down: %s \n", url, err), 0
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		fmt.Printf("✅ %s is up \n", url)
	}

	var icon string
	if resp.StatusCode == http.StatusOK {
		icon = "✅"
	} else {
		icon = "⚠️"
	}

	return fmt.Sprintf("%s %s returned status code %d \n", icon, url, resp.StatusCode), resp.StatusCode
}
