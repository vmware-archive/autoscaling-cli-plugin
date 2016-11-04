package mocks

import (
	"encoding/json"
	"fmt"
)

type DoCall struct {
	Receives struct {
		Method       string
		URL          string
		RequestData  interface{}
		ResponseData interface{}
	}
	Returns struct {
		Error error
	}

	ResponseJSON string
}

type JSONClient struct {
	DoCallCount int
	DoCalls     map[int]*DoCall
}

func NewJSONClient(callCount int) *JSONClient {
	doCalls := map[int]*DoCall{}

	for i := 0; i < callCount; i++ {
		doCalls[i] = &DoCall{}
	}

	return &JSONClient{DoCalls: doCalls}
}

func (c *JSONClient) Do(method string, url string, requestData interface{}, responseData interface{}) error {
	call := c.DoCalls[c.DoCallCount]
	defer func() { c.DoCallCount++ }()

	call.Receives.Method = method
	call.Receives.URL = url
	call.Receives.RequestData = requestData
	call.Receives.ResponseData = responseData

	if responseData != nil {
		err := json.Unmarshal([]byte(call.ResponseJSON), responseData)
		if err != nil {
			return fmt.Errorf("Your FAKE response JSON couldn't be unmarshalled: %s", err)
		}
	}

	return call.Returns.Error
}
