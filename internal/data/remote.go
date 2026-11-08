package data

import (
	"mygo-app/internal/data/remote"
	"mygo-app/internal/domain"
)

// OpenRemote connects to a remote REST API service and returns a domain.Repo.
func OpenRemote(baseURL, token string) domain.Repo {
	return remote.Open(baseURL, token)
}

// Client is a type alias for remote.Client, preserving backward compatibility.
type Client = remote.Client

// APIError is a type alias for remote.APIError, preserving backward compatibility.
type APIError = remote.APIError

// NewClient creates a new remote client, preserving backward compatibility.
func NewClient(baseURL, token string) *Client {
	return remote.NewClient(baseURL, token)
}
