package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func addMSEdgeBlockList() error {
	const url = "https://edge.microsoft.com/abusiveadblocking/api/v1/blocklist"
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("error fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	var blocklist struct {
		Sites []struct {
			Url string `json:"url"`
		} `json:"sites"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&blocklist); err != nil {
		return err
	}
	for _, site := range blocklist.Sites {
		hosts[site.Url] = struct{}{}
		hosts["www."+site.Url] = struct{}{}
	}

	fmt.Printf("Microsoft Edge blocklist: found %d hosts", len(blocklist.Sites))

	return nil
}
