package main

import (
	"fmt"
	"github.com/getzep/zep-go/zep"
	"net/http"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
)

const (
	APIBasePath           = "/api/v1"
	ServerErrorMessage    = "Failed to connect to Zep server. Please check that the server is running, the API URL is correct, and no other process is using the same port"
	MinServerVersion      = "0.16.0"
	MinServerWarningMsg   = "You are using an incompatible Zep server version. Please upgrade to " + MinServerVersion + " or later."
	DefaultRequestTimeout = 30 // In seconds
)

var _ ZepClient = &DefaultZepClient{}

type ZepClient interface {
	GetFullURL(endpoint string) string
	CheckServer() error
	HandleRequest(requestPromise *http.Request, notFoundMessage string) (*http.Response, error)
}

// NewZepClient creates a new ZepClient. If client is provided, it will be used to make requests.
// Otherwise, a default client will be created with a 30 second timeout.
func NewZepClient(serverURL string, apiKey string, client *http.Client) *DefaultZepClient {
	headers := make(map[string]string)
	if apiKey != "" {
		headers["Authorization"] = "Bearer " + apiKey
	}
	if client == nil {
		client = &http.Client{Timeout: DefaultRequestTimeout * time.Second}
	}

	// Remove trailing slash from server URL
	serverURL = strings.TrimSuffix(serverURL, "/")

	c := &DefaultZepClient{ServerURL: serverURL, Headers: headers, Client: client}
	err := c.CheckServer()
	if err != nil {
		fmt.Println(err)
	}

	return c
}

// DefaultZepClient is the implementation of ZepClient.
type DefaultZepClient struct {
	ServerURL string
	Headers   map[string]string
	Client    *http.Client
}

// GetFullURL returns the full URL for the given endpoint.
// It concatenates the server URL, API base path, and endpoint.
func (z *DefaultZepClient) GetFullURL(endpoint string) string {
	return joinPaths(z.ServerURL, APIBasePath, endpoint)
}

// CheckServer checks if the server is running and returns an error if it is not.
// It also checks if the server version is compatible with this client.
func (z *DefaultZepClient) CheckServer() error {
	healthCheck := "/healthz"
	healthCheckURL := z.ServerURL + healthCheck

	req, err := http.NewRequest("GET", healthCheckURL, nil)
	if err != nil {
		return err
	}
	for key, value := range z.Headers {
		req.Header.Add(key, value)
	}

	resp, err := z.Client.Do(req)
	if err != nil {
		return err
	}

	zepServerVersion := resp.Header.Get("X-Zep-Version")
	meetsMinVersion, err := isVersionGreaterOrEqual(zepServerVersion)
	if err != nil {
		return err
	}
	if !meetsMinVersion {
		fmt.Println("Warning: " + MinServerWarningMsg)
	}
	if resp.StatusCode != 200 {
		return &zep.ZepError{Message: ServerErrorMessage}
	}

	return nil
}

// HandleRequest makes the request and returns the response if the request is successful.
// If the request is not successful, it returns an appropriate error:
// - NotFoundError if the status code is 404
// - AuthenticationError if the status code is 401
// - APIError if the status code is anything else
func (z *DefaultZepClient) HandleRequest(requestPromise *http.Request, notFoundMessage string) (*http.Response, error) {
	response, err := z.Client.Do(requestPromise)
	if err != nil {
		return nil, &zep.ZepError{Message: ServerErrorMessage + ": " + err.Error()}
	}

	switch response.StatusCode {
	case http.StatusOK:
		return response, nil
	case http.StatusNotFound:
		return nil, &zep.NotFoundError{ZepError: zep.ZepError{Message: notFoundMessage}}
	case http.StatusUnauthorized:
		return nil, &zep.AuthenticationError{ZepError: zep.ZepError{Message: "Authentication failed."}}
	default:
		return nil, &zep.APIError{ZepError: zep.ZepError{Message: fmt.Sprintf("Got an unexpected status code: %d", response.StatusCode)}}
	}
}

func isVersionGreaterOrEqual(version string) (bool, error) {
	c, err := semver.NewConstraint(">= " + MinServerVersion)
	if err != nil {
		return false, err
	}
	currentVersion, err := semver.NewVersion(version)
	if err != nil {
		return false, err
	}
	return c.Check(currentVersion), nil
}

func joinPaths(paths ...string) string {
	return strings.Join(paths, "/")
}
