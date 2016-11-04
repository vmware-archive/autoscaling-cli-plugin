package plugin_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/phopper-pivotal/autoscaling-cli-plugin/mocks"
	"github.com/phopper-pivotal/autoscaling-cli-plugin/plugin"
)

var _ = Describe("JSON Client", func() {
	Describe("Do", func() {
		var (
			jsonClient plugin.JSONClient
			httpClient *mocks.HTTPClient

			responseData map[string]string
		)

		BeforeEach(func() {
			httpClient = &mocks.HTTPClient{}
			httpClient.DoCall.Returns.Errors = make([]error, 1)
			httpClient.DoCall.Returns.Responses = []*http.Response{
				&http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Body:       ioutil.NopCloser(strings.NewReader(`{"some-key": "some-value"}`)),
				},
			}

			jsonClient = plugin.JSONClient{
				httpClient,
				"some-token",
			}
		})

		It("should make a request to the HTTP server", func() {
			requestData := map[string]string{
				"bananas": "tasty",
			}

			err := jsonClient.Do("POST", "/some/url", requestData, &responseData)
			Expect(err).NotTo(HaveOccurred())
			Expect(ioutil.ReadAll(httpClient.DoCall.Receives.Request.Body)).To(Equal([]byte(`{"bananas":"tasty"}`)))
			Expect(responseData).To(HaveKeyWithValue("some-key", "some-value"))
		})

		Context("when a requestData variable is not provided", func() {
			It("should not attempt to marshal the request", func() {
				err := jsonClient.Do("GET", "http://example.com/some/url", nil, &responseData)
				Expect(err).NotTo(HaveOccurred())
				Expect(httpClient.DoCall.Receives.Request.Body).To(BeNil())
				Expect(httpClient.DoCall.Receives.Request.URL.Path).To(Equal("/some/url"))
			})
		})

		Context("when a responseData variable is not provided", func() {
			It("should not attempt to unmarshal the response", func() {
				httpClient.DoCall.Returns.Responses[0].Body = ioutil.NopCloser(strings.NewReader(`{{{`))
				err := jsonClient.Do("GET", "http://example.com/some/url", nil, nil)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("failure cases", func() {
			Context("when the request is invalid", func() {
				It("returns an error", func() {
					err := jsonClient.Do("GET", "http://example.com/%%%", nil, nil)
					Expect(err).To(HaveOccurred())
					Expect(httpClient.DoCall.Receives.Request).To(BeNil())
				})
			})

			Context("when the http client errors", func() {
				It("returns an error", func() {
					httpClient.DoCall.Returns.Errors = []error{errors.New("some error")}

					err := jsonClient.Do("GET", "some-url", nil, nil)
					Expect(err).To(MatchError("some error"))
				})
			})

			Context("when the server responds with an unexpected status code", func() {
				It("returns an error", func() {
					httpClient.DoCall.Returns.Responses[0].StatusCode = http.StatusTeapot
					httpClient.DoCall.Returns.Responses[0].Status = "418 TEAPOT!!"

					err := jsonClient.Do("GET", "some-url", nil, nil)
					Expect(err).To(MatchError("unexpected response code: 418 TEAPOT!!"))
				})
			})

			Context("when we expect a JSON response but the server sends back invalid JSON", func() {
				It("returns an error", func() {
					httpClient.DoCall.Returns.Responses[0].Body = ioutil.NopCloser(bytes.NewReader([]byte(`{{{`)))

					var responseData string
					err := jsonClient.Do("GET", "some-url", nil, &responseData)
					Expect(err).To(MatchError(ContainSubstring("couldn't parse response: invalid character")))
				})
			})
		})
	})
})
