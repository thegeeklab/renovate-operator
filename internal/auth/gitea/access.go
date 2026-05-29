package gitea

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const defaultPageSize = 50

var errUnexpectedStatus = errors.New("unexpected status code")

type GiteaAccessChecker struct {
	forgeURL   string
	token      string
	httpClient *http.Client
}

func NewGiteaAccessChecker(forgeURL, token string, httpClient *http.Client) *GiteaAccessChecker {
	return &GiteaAccessChecker{
		forgeURL:   strings.TrimRight(forgeURL, "/"),
		token:      token,
		httpClient: httpClient,
	}
}

func (c *GiteaAccessChecker) GetAccessibleRepos(ctx context.Context) (map[string]bool, error) {
	result := make(map[string]bool)

	page := 1

	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet,
			fmt.Sprintf("%s/api/v1/repos/search?limit=%d&page=%d", c.forgeURL, defaultPageSize, page), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", "token "+c.token)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch repos: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()

			return nil, fmt.Errorf("%w: %d", errUnexpectedStatus, resp.StatusCode)
		}

		var respBody struct {
			OK   bool        `json:"ok"`
			Data []giteaRepo `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
			resp.Body.Close()

			return nil, fmt.Errorf("failed to decode repos: %w", err)
		}

		resp.Body.Close()

		for _, repo := range respBody.Data {
			result[repo.FullName] = true
		}

		if len(respBody.Data) == 0 {
			break
		}

		page++
	}

	return result, nil
}

type giteaRepo struct {
	FullName string `json:"full_name"`
}
