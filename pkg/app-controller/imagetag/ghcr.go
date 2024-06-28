package imagetag

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)



type GHCRClient struct {
	baseURL    string
	httpClient *http.Client
	token      string

}

func NewGHCRClient(token string) *GHCRClient {
	const ghcrBaseURL = "https://ghcr.io"
	return &GHCRClient{
		baseURL:    ghcrBaseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		token:      token,
		
	}
}

func (rc *GHCRClient) GetTags(imageName string) ([]string, error) {
	url := fmt.Sprintf("%s/v2/%s/tags/list", rc.baseURL, imageName)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+rc.token)

	resp, err := rc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get tags: %s", resp.Status)
	}

	var result struct {
		Tags []string `json:"tags"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}

	return result.Tags, nil
}