package brev_api

import (
	"github.com/brevdev/brev-cli/pkg/requests"
)

type User struct {
	Id         string   `json:"id"`

}

func (a *Agent) GetMe() (*User, error) {
	request := requests.RESTRequest{
		Method:   "GET",
		Endpoint: brevEndpoint("/api/me"),
		QueryParams: []requests.QueryParam{
			{"utm_source", "cli"},
		},
		Headers: []requests.Header{
			{"Authorization", "Bearer " + a.Key.AccessToken},
		},
	}
	response, err := request.SubmitStrict()
	if err != nil {
		return nil, err
	}

	var payload User
	err = response.UnmarshalPayload(&payload)
	if err != nil {
		return nil, err
	}

	return &payload, nil
}