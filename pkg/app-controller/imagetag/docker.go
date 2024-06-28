package imagetag

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type DockerHubClient struct {
	baseURL    string
	httpClient *http.Client
	token      string
}

func NewDockerHubClient(token string) *DockerHubClient {
	const dockerHubBaseURL = "https://registry.hub.docker.com"
	return &DockerHubClient{
		baseURL:    dockerHubBaseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		token:      token,
	}
}

func (rc *DockerHubClient) GetTags(imageName string) ([]string, error) {
	url := fmt.Sprintf("%s/v2/repositories/%s/tags", rc.baseURL, imageName)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Include the token in the Authorization header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", rc.token))

	resp, err := rc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get tags: %s", resp.Status)
	}

	var result struct {
		Results []struct {
			Name string `json:"name"`
		} `json:"results"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}

	var tags []string
	for _, tag := range result.Results {
		tags = append(tags, tag.Name)
	}

	return tags, nil
}
