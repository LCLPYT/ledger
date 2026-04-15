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

// FetchUsername fetches the current Minecraft username for the given UUID.
// Returns ("", nil) when the profile is not found (404).
func FetchUsername(uuid string) (string, error) {
	bare := strings.ReplaceAll(uuid, "-", "")
	url := "https://api.minecraftservices.com/minecraft/profile/lookup/" + bare
	resp, err := httpClient.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("mojang API returned %d", resp.StatusCode)
	}
	var p profileResponse
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return "", err
	}
	return p.Name, nil
}

// FetchUUIDByName fetches the UUID for a Minecraft username.
// Returns ("", nil) when the player is not found (404).
// Returned UUID is formatted with dashes (8-4-4-4-12).
func FetchUUIDByName(name string) (string, error) {
	url := "https://api.mojang.com/minecraft/profile/lookup/name/" + name
	resp, err := httpClient.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("mojang API returned %d", resp.StatusCode)
	}
	var p profileResponse
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return "", err
	}
	// Format bare hex UUID with dashes
	id := p.ID
	return id[0:8] + "-" + id[8:12] + "-" + id[12:16] + "-" + id[16:20] + "-" + id[20:], nil
}
