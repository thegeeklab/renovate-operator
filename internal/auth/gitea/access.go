package gitea

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v5"
)

const (
	defaultPageSize  = 50
	maxPages         = 100
	maxFetchTimeout  = 2 * time.Minute
	repoCheckTimeout = 10 * time.Second

	backoffInitial    = 200 * time.Millisecond
	backoffMax        = 10 * time.Second
	backoffMultiplier = 2.0
	backoffMaxTries   = 3
)

var (
	errUnexpectedStatus = errors.New("unexpected status code")
	errServerError      = errors.New("server error")
)

type GiteaAccessChecker struct {
	forgeURL   string
	token      string
	httpClient *http.Client
}

//nolint:tagliatelle // Gitea API uses snake_case
type giteaRepo struct {
	FullName string `json:"full_name"`
}

func NewGiteaAccessChecker(forgeURL, token string, httpClient *http.Client) *GiteaAccessChecker {
	return &GiteaAccessChecker{
		forgeURL:   strings.TrimRight(forgeURL, "/"),
		token:      token,
		httpClient: httpClient,
	}
}

func (c *GiteaAccessChecker) GetAccessibleRepos(ctx context.Context) (map[string]bool, error) {
	ctx, cancel := context.WithTimeout(ctx, maxFetchTimeout)
	defer cancel()

	result := make(map[string]bool)

	page := 1

	for {
		if err := ctx.Err(); err != nil {
			if len(result) > 0 {
				return result, fmt.Errorf("fetch repos cancelled with partial results: %w", err)
			}

			return result, fmt.Errorf("fetch repos cancelled: %w", err)
		}

		data, err := c.fetchPageWithRetry(ctx, page)
		if err != nil {
			if len(result) > 0 {
				return result, fmt.Errorf("fetch failed with partial results: %w", err)
			}

			return result, err
		}

		for _, repo := range data {
			result[repo.FullName] = true
		}

		if len(data) == 0 || page >= maxPages {
			break
		}

		page++
	}

	return result, nil
}

func (c *GiteaAccessChecker) fetchPageWithRetry(ctx context.Context, page int) ([]giteaRepo, error) {
	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = backoffInitial
	bo.MaxInterval = backoffMax
	bo.Multiplier = backoffMultiplier

	return backoff.Retry(ctx, func() ([]giteaRepo, error) {
		data, statusCode, retryAfter, err := c.fetchPage(ctx, page)
		if err != nil {
			return nil, err
		}

		if statusCode == http.StatusOK {
			return data, nil
		}

		if statusCode != http.StatusTooManyRequests && statusCode < 500 {
			return nil, backoff.Permanent(fmt.Errorf("%w: %d", errUnexpectedStatus, statusCode))
		}

		if statusCode == http.StatusTooManyRequests && retryAfter > 0 {
			return nil, backoff.RetryAfter(int(retryAfter.Seconds()))
		}

		return nil, fmt.Errorf("%w: %d", errServerError, statusCode)
	}, backoff.WithBackOff(bo), backoff.WithMaxTries(backoffMaxTries))
}

func (c *GiteaAccessChecker) fetchPage(ctx context.Context, page int) ([]giteaRepo, int, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/api/v1/repos/search?limit=%d&page=%d", c.forgeURL, defaultPageSize, page), nil)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to fetch repos: %w", err)
	}

	defer resp.Body.Close()

	retryAfter := c.parseRetryAfter(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, retryAfter, nil
	}

	var respBody struct {
		OK   bool        `json:"ok"`
		Data []giteaRepo `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return nil, resp.StatusCode, 0, fmt.Errorf("failed to decode repos: %w", err)
	}

	return respBody.Data, resp.StatusCode, 0, nil
}

func (c *GiteaAccessChecker) parseRetryAfter(resp *http.Response) time.Duration {
	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter == "" {
		return 0
	}

	if seconds, err := strconv.Atoi(retryAfter); err == nil {
		return time.Duration(seconds) * time.Second
	}

	if t, err := http.ParseTime(retryAfter); err == nil {
		return time.Until(t)
	}

	return 0
}

func (c *GiteaAccessChecker) IsRepoAccessible(ctx context.Context, fullName string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, repoCheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/api/v1/repos/%s", c.forgeURL, fullName), nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to check repo access: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
		return false, nil
	}

	return false, fmt.Errorf("%w: %d", errUnexpectedStatus, resp.StatusCode)
}
