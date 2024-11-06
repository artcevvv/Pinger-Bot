package main

import (
	"fmt"
	"net/http"
)

func pingURL(url string) (string, int) {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Sprintf("Failed to ping %s: %s", url, err.Error()), http.StatusInternalServerError
	}
	defer resp.Body.Close()

	return fmt.Sprintf("URL: %s Status: %s", url, resp.Status), resp.StatusCode
}
