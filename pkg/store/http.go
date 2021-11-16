package store

import (
	"github.com/brevdev/brev-cli/pkg/brevapi"
	resty "github.com/go-resty/resty/v2"
)

type NoAuthHTTPStore struct {
	FileStore
	noAuthHTTPClient *NoAuthHTTPClient
}

func (f *FileStore) WithNoAuthHTTPClient(c *NoAuthHTTPClient) *NoAuthHTTPStore {
	return &NoAuthHTTPStore{*f, c}
}

type NoAuthHTTPClient resty.Client

func NewNoAuthHTTPClient() *NoAuthHTTPClient {
	return (*NoAuthHTTPClient)(resty.New())
}

type AuthHTTPStore struct {
	NoAuthHTTPStore
	authHTTPClient *AuthHTTPClient
}

func (f *FileStore) WithAuthHTTPClient(c *AuthHTTPClient) *AuthHTTPStore {
	na := f.WithNoAuthHTTPClient(NewNoAuthHTTPClient()) // TODO pull from auth client
	return &AuthHTTPStore{*na, c}
}

func (n *NoAuthHTTPStore) WithAuthHTTPClient(c *AuthHTTPClient) *AuthHTTPStore {
	return &AuthHTTPStore{*n, c}
}

type AuthHTTPClient struct {
	restyClient       *resty.Client
	toDeprecateClient *brevapi.Client
}

func NewAuthHTTPClient(client *brevapi.Client) *AuthHTTPClient {
	restyClient := resty.New()
	return &AuthHTTPClient{restyClient, client}
}
