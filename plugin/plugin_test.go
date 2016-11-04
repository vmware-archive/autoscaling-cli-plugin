package plugin_test

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/cli/plugin/models"
	"github.com/phopper-pivotal/autoscaling-cli-plugin/mocks"
	"github.com/phopper-pivotal/autoscaling-cli-plugin/plugin"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type autoscalingBinding struct {
	AppGuid         string `json:"app_guid"`
	MinInstances    int    `json:"min_instances"`
	MaxInstances    int    `json:"max_instances"`
	CPUMinThreshold int    `json:"cpu_min_threshold"`
	CPUMaxThreshold int    `json:"cpu_max_threshold"`
	Enabled         bool   `json:"enabled"`
}

var _ = Describe("Plugin", func() {
	Describe("FetchCLIDependencies", func() {
		var (
			p             *plugin.Plugin
			cliConnection *mocks.CLIConnection
			args          []string
		)

		BeforeEach(func() {
			p = plugin.NewPlugin()
			cliConnection = &mocks.CLIConnection{}
			args = []string{"app-name", "service-name"}

			cliConnection.IsLoggedInCall.Returns.LoggedIn = true
			cliConnection.GetServiceCall.Returns.Service.DashboardUrl = "dashboard.example.com/something-that-doesnot-matter"
			cliConnection.GetServiceCall.Returns.Service.Guid = "some-service-instance-guid"
			cliConnection.GetAppCall.Returns.App.Guid = "some-app-guid"
			cliConnection.ApiEndpointCall.Returns.ApiEndpoint = "api.example.com"
			cliConnection.AccessTokenCall.Returns.Token = "bearer some-token"
			cliConnection.IsSSLDisabledCall.Returns.Disabled = true
		})

		It("returns all CLI dependency values", func() {
			dependencies, err := p.FetchCLIDependencies(cliConnection, args)
			Expect(err).NotTo(HaveOccurred())
			Expect(dependencies).To(Equal(plugin.CLIDependencies{
				AccessToken: "bearer some-token",
				AppName:     "app-name",
				ServiceName: "service-name",
				APIEndpoint: "api.example.com",
				Service: plugin_models.GetService_Model{
					Guid:         "some-service-instance-guid",
					DashboardUrl: "dashboard.example.com/something-that-doesnot-matter",
				},
				App: plugin_models.GetAppModel{
					Guid: "some-app-guid",
				},
				JSONClient: &plugin.JSONClient{
					HTTPClient: &http.Client{
						Transport: &http.Transport{
							TLSClientConfig: &tls.Config{
								InsecureSkipVerify: true,
							},
						},
					},
					AccessToken: "bearer some-token",
				},
			}))

			Expect(cliConnection.GetServiceCall.Receives.ServiceName).To(Equal("service-name"))
			Expect(cliConnection.GetAppCall.Receives.AppName).To(Equal("app-name"))
		})

		Context("failure cases", func() {
			Context("when the user does not provide an app name or service instance name", func() {
				It("returns an error", func() {
					_, err := p.FetchCLIDependencies(cliConnection, []string{"app-name"})
					Expect(err).To(MatchError("provide APP_NAME and SERVICE_NAME on command line"))

					_, err = p.FetchCLIDependencies(cliConnection, []string{"app-name", "service-name", "another-arg"})
					Expect(err).To(MatchError("too many arguments provided"))
				})
			})

			Context("when checking if the user is logged in returns an error", func() {
				It("returns an error", func() {
					cliConnection.IsLoggedInCall.Returns.Error = errors.New("some error")

					_, err := p.FetchCLIDependencies(cliConnection, args)
					Expect(err).To(MatchError("some error"))
				})
			})

			Context("when the user is not logged in", func() {
				It("returns an error", func() {
					cliConnection.IsLoggedInCall.Returns.LoggedIn = false

					_, err := p.FetchCLIDependencies(cliConnection, args)
					Expect(err).To(MatchError("you need to log in"))
				})
			})

			Context("when the access token couldn't be retrieved", func() {
				It("returns an error", func() {
					cliConnection.AccessTokenCall.Returns.Error = errors.New("some error")

					_, err := p.FetchCLIDependencies(cliConnection, args)
					Expect(err).To(MatchError("couldn't get access token: some error"))
				})
			})

			Context("when fetching the service fails", func() {
				It("returns an error", func() {
					cliConnection.GetServiceCall.Returns.Error = errors.New("some error")

					_, err := p.FetchCLIDependencies(cliConnection, args)
					Expect(err).To(MatchError("couldn't get service named service-name: some error"))
				})
			})

			Context("when the API end-point cannot be retrieved", func() {
				It("returns a helpful error", func() {
					cliConnection.ApiEndpointCall.Returns.Error = errors.New("failed to get endpoint")

					_, err := p.FetchCLIDependencies(cliConnection, args)
					Expect(err).To(MatchError("couldn't get API end-point: failed to get endpoint"))
				})
			})

			Context("when the app cannot be retrieved", func() {
				It("returns an error", func() {
					cliConnection.GetAppCall.Returns.Error = errors.New("failed to get app")

					_, err := p.FetchCLIDependencies(cliConnection, args)
					Expect(err).To(MatchError("couldn't get app app-name: failed to get app"))
				})
			})

			Context("when checking if ssl verification is disabled fails", func() {
				It("returns an error", func() {
					cliConnection.IsSSLDisabledCall.Returns.Error = errors.New("something failed")

					_, err := p.FetchCLIDependencies(cliConnection, args)
					Expect(err).To(MatchError("couldn't check if ssl verification is disabled: something failed"))
				})
			})
		})
	})

	Describe("RunWithError", func() {
		var (
			p            *plugin.Plugin
			jsonClient   *mocks.JSONClient
			dependencies plugin.CLIDependencies
			flags        plugin.Flags
		)

		BeforeEach(func() {
			p = plugin.NewPlugin()
			jsonClient = mocks.NewJSONClient(3)

			jsonClient.DoCalls[0].ResponseJSON = `{
				"Resources": [
					{
						"Metadata": {
							"GUID": "some-service-binding-guid"
						}
					}
				]
			}`

			jsonClient.DoCalls[1].ResponseJSON = `{
				"min_instances": 3,
				"max_instances": 7,
				"cpu_min_threshold": 20,
				"cpu_max_threshold": 80,
				"enabled": false
			}`

			dependencies = plugin.CLIDependencies{
				AccessToken: "bearer some-token",
				AppName:     "app-name",
				ServiceName: "service-name",
				Service: plugin_models.GetService_Model{
					Guid:         "some-service-instance-guid",
					DashboardUrl: fmt.Sprintf("%s/something-that-doesnot-matter", "http://autoscaling.example.com"),
				},
				APIEndpoint: "https://cloudcontroller.example.com",
				App: plugin_models.GetAppModel{
					Guid: "some-app-guid",
				},
				JSONClient: jsonClient,
			}

			flags = plugin.Flags{
				MinInstances:    9,
				MaxInstances:    30,
				CPUMinThreshold: 10,
				CPUMaxThreshold: 90,
			}
		})

		It("gets the service binding GUID from cloud controller", func() {
			Expect(p.RunWithError(dependencies, flags)).To(Succeed())
			Expect(jsonClient.DoCalls[0].Receives.Method).To(Equal("GET"))
			Expect(jsonClient.DoCalls[0].Receives.URL).To(Equal("https://cloudcontroller.example.com/v2/service_bindings?q=app_guid%3Asome-app-guid&q=service_instance_guid%3Asome-service-instance-guid"))
			Expect(jsonClient.DoCalls[0].Receives.RequestData).To(BeNil())
		})

		It("gets the gets the service binding info from autoscaling", func() {
			Expect(p.RunWithError(dependencies, flags)).To(Succeed())
			Expect(jsonClient.DoCalls[1].Receives.Method).To(Equal("GET"))
			Expect(jsonClient.DoCalls[1].Receives.URL).To(Equal("http://autoscaling.example.com/api/bindings/some-service-binding-guid"))
			Expect(jsonClient.DoCalls[1].Receives.RequestData).To(BeNil())
		})

		It("enables the autoscaling service binding", func() {
			Expect(p.RunWithError(dependencies, flags)).To(Succeed())
			Expect(jsonClient.DoCalls[2].Receives.Method).To(Equal("POST"))
			Expect(jsonClient.DoCalls[2].Receives.URL).To(Equal("http://autoscaling.example.com/api/bindings/some-service-binding-guid"))
			Expect(jsonClient.DoCalls[2].Receives.RequestData).To(Equal(&plugin.AutoscalingBinding{
				AppGuid:         "some-app-guid",
				MinInstances:    9,
				MaxInstances:    30,
				CPUMinThreshold: 10,
				CPUMaxThreshold: 90,
				Enabled:         true,
			}))
		})

		Context("when no arguements are specified on the cli", func() {
			BeforeEach(func() {
				flags = plugin.Flags{}
			})

			It("just enables the service without changing any binding parameters", func() {
				Expect(p.RunWithError(dependencies, flags)).To(Succeed())
				Expect(jsonClient.DoCalls[2].Receives.Method).To(Equal("POST"))
				Expect(jsonClient.DoCalls[2].Receives.URL).To(Equal("http://autoscaling.example.com/api/bindings/some-service-binding-guid"))
				Expect(jsonClient.DoCalls[2].Receives.RequestData).To(Equal(&plugin.AutoscalingBinding{
					AppGuid:         "some-app-guid",
					MinInstances:    3,
					MaxInstances:    7,
					CPUMinThreshold: 20,
					CPUMaxThreshold: 80,
					Enabled:         true,
				}))
			})

			Context("when MinInstances > MaxInstances", func() {
				It("should return an error", func() {
					flags.MinInstances = 35
					flags.MaxInstances = 34

					Expect(p.RunWithError(dependencies, flags)).To(MatchError("min instances must be <= max instances"))
				})
			})

			Context("when CPUMinThreshold > CPUMaxThreshold", func() {
				It("should return an error", func() {
					flags.CPUMinThreshold = 75
					flags.CPUMaxThreshold = 24

					Expect(p.RunWithError(dependencies, flags)).To(MatchError("CPU min threshold must be <= CPU max threshold"))
				})
			})
		})

		Context("error cases", func() {
			Context("when we can't construct a url to query cloud controller", func() {
				It("should return the error", func() {
					dependencies.APIEndpoint = "%%%"

					err := p.RunWithError(dependencies, flags)
					Expect(err).To(MatchError("invalid API URL from cli: %%%"))
				})
			})

			Context("when the request to cloud controller fails", func() {
				It("should return the error", func() {
					jsonClient.DoCalls[0].Returns.Error = errors.New("cc call failed")

					err := p.RunWithError(dependencies, flags)
					Expect(err).To(MatchError("couldn't retrieve service binding: cc call failed"))
				})
			})

			Context("when cloud controller doesn't return a resource", func() {
				It("should return an error", func() {
					jsonClient.DoCalls[0].ResponseJSON = `{"Resources": []}`

					err := p.RunWithError(dependencies, flags)
					Expect(err).To(MatchError("couldn't find service binding for app-name to service-name"))
				})
			})

			Context("when we can't construct a url to query autoscaling", func() {
				It("should return the error", func() {
					dependencies.Service.DashboardUrl = "%%%"

					err := p.RunWithError(dependencies, flags)
					Expect(err).To(MatchError("invalid dashboard URL from service instance: %%%"))
				})
			})

			Context("when the GET request to autoscaling fails", func() {
				It("should return the error", func() {
					jsonClient.DoCalls[1].Returns.Error = errors.New("autoscaling GET call failed")

					err := p.RunWithError(dependencies, flags)
					Expect(err).To(MatchError("autoscaling API: autoscaling GET call failed"))
				})
			})

			Context("when the POST request to autoscaling fails", func() {
				It("should return the error", func() {
					jsonClient.DoCalls[2].Returns.Error = errors.New("autoscaling POST call failed")

					err := p.RunWithError(dependencies, flags)
					Expect(err).To(MatchError("autoscaling API: autoscaling POST call failed"))
				})
			})
		})
	})
})
