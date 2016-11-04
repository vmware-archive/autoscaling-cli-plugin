package mocks

import "net/http"

type HTTPClient struct {
	DoCall struct {
		CallCount int
		Receives  struct {
			Request *http.Request
		}
		Returns struct {
			Responses []*http.Response
			Errors    []error
		}
	}
}

func (c *HTTPClient) Do(request *http.Request) (*http.Response, error) {
	c.DoCall.Receives.Request = request
	callCount := c.DoCall.CallCount
	c.DoCall.CallCount++

	return c.DoCall.Returns.Responses[callCount], c.DoCall.Returns.Errors[callCount]
}
