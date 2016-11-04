package plugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type JSONClient struct {
	HTTPClient  httpClient
	AccessToken string
}

func (c JSONClient) Do(method string, url string, requestData interface{}, responseData interface{}) error {
	var requestBodyReader io.Reader
	if requestData != nil {
		requestBytes, err := json.Marshal(requestData)
		if err != nil {
			return err // not tested
		}

		requestBodyReader = bytes.NewReader(requestBytes)
	}

	request, err := http.NewRequest(method, url, requestBodyReader)
	if err != nil {
		return err
	}

	if requestData != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	request.Header.Set("Authorization", c.AccessToken)

	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response code: %s", response.Status)
	}

	if responseData != nil {
		if err = json.NewDecoder(response.Body).Decode(&responseData); err != nil {
			return fmt.Errorf("couldn't parse response: %s", err)
		}
	}

	return nil
}
