package mojang

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 5 * time.Second}

type profileResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// fetchProfile GETs url and decodes the JSON response into profileResponse.
// Returns (nil, nil) on 404.
func fetchProfile(url string) (*profileResponse, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mojang API returned %d", resp.StatusCode)
	}
	var p profileResponse
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, err
	}
	return &p, nil
}

// FetchUsername fetches the current Minecraft username for the given UUID.
// Returns ("", nil) when the profile is not found (404).
func FetchUsername(uuid string) (string, error) {
	bare := strings.ReplaceAll(uuid, "-", "")
	p, err := fetchProfile("https://api.minecraftservices.com/minecraft/profile/lookup/" + bare)
	if err != nil || p == nil {
		return "", err
	}
	return p.Name, nil
}

// FetchUUIDByName fetches the UUID for a Minecraft username.
// Returns ("", nil) when the player is not found (404).
// Returned UUID is formatted with dashes (8-4-4-4-12).
func FetchUUIDByName(name string) (string, error) {
	p, err := fetchProfile("https://api.mojang.com/minecraft/profile/lookup/name/" + name)
	if err != nil || p == nil {
		return "", err
	}
	id := p.ID
	return id[0:8] + "-" + id[8:12] + "-" + id[12:16] + "-" + id[16:20] + "-" + id[20:], nil
}
