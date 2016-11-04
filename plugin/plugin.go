package plugin

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"code.cloudfoundry.org/cli/plugin"
	"code.cloudfoundry.org/cli/plugin/models"
)

func NewPlugin() *Plugin {
	return &Plugin{}
}

type Plugin struct{}

type AutoscalingBinding struct {
	AppGuid         string `json:"app_guid"`
	MinInstances    int    `json:"min_instances"`
	MaxInstances    int    `json:"max_instances"`
	CPUMinThreshold int    `json:"cpu_min_threshold"`
	CPUMaxThreshold int    `json:"cpu_max_threshold"`
	Enabled         bool   `json:"enabled"`
}

type cliConnection interface {
	IsLoggedIn() (bool, error)
	AccessToken() (string, error)
	ApiEndpoint() (string, error)
	GetService(name string) (plugin_models.GetService_Model, error)
	GetApp(name string) (plugin_models.GetAppModel, error)
	IsSSLDisabled() (bool, error)
}

type httpClient interface {
	Do(*http.Request) (*http.Response, error)
}

type jsonClient interface {
	Do(method string, url string, requestData interface{}, responseData interface{}) error
}

type CLIDependencies struct {
	AccessToken string
	AppName     string
	ServiceName string
	Service     plugin_models.GetService_Model
	APIEndpoint string
	App         plugin_models.GetAppModel
	JSONClient  jsonClient
}

func (p *Plugin) FetchCLIDependencies(cliConnection plugin.CliConnection, args []string) (CLIDependencies, error) {
	if len(args) < 2 {
		return CLIDependencies{}, fmt.Errorf("provide APP_NAME and SERVICE_NAME on command line")
	}

	if len(args) > 2 {
		return CLIDependencies{}, fmt.Errorf("too many arguments provided")
	}

	appName := args[0]
	serviceName := args[1]

	isLoggedIn, err := cliConnection.IsLoggedIn()
	if err != nil {
		return CLIDependencies{}, err
	}
	if !isLoggedIn {
		return CLIDependencies{}, fmt.Errorf("you need to log in")
	}

	accessToken, err := cliConnection.AccessToken()
	if err != nil {
		return CLIDependencies{}, fmt.Errorf("couldn't get access token: %s", err)
	}

	service, err := cliConnection.GetService(serviceName)
	if err != nil {
		return CLIDependencies{}, fmt.Errorf("couldn't get service named %s: %s", serviceName, err)
	}

	apiEndpoint, err := cliConnection.ApiEndpoint()
	if err != nil {
		return CLIDependencies{}, fmt.Errorf("couldn't get API end-point: %s", err)
	}

	app, err := cliConnection.GetApp(appName)
	if err != nil {
		return CLIDependencies{}, fmt.Errorf("couldn't get app %s: %s", appName, err)
	}

	skipVerifySSL, err := cliConnection.IsSSLDisabled()
	if err != nil {
		return CLIDependencies{}, fmt.Errorf("couldn't check if ssl verification is disabled: %s", err)
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: skipVerifySSL,
			},
		},
	}

	jsonClient := &JSONClient{
		HTTPClient:  httpClient,
		AccessToken: accessToken,
	}

	return CLIDependencies{
		AccessToken: accessToken,
		AppName:     appName,
		ServiceName: serviceName,
		Service:     service,
		APIEndpoint: apiEndpoint,
		App:         app,
		JSONClient:  jsonClient,
	}, nil
}

func getCCQueryURL(apiEndpoint, appGUID, serviceInstanceGUID string) (string, error) {
	serviceBindingsURL, err := url.Parse(apiEndpoint)
	if err != nil {
		return "", fmt.Errorf("invalid API URL from cli: %s", apiEndpoint)
	}

	serviceBindingsURL.Path = "/v2/service_bindings"
	serviceBindingsURL.RawQuery = url.Values{
		"q": []string{
			fmt.Sprintf("app_guid:%s", appGUID),
			fmt.Sprintf("service_instance_guid:%s", serviceInstanceGUID),
		},
	}.Encode()

	return serviceBindingsURL.String(), nil
}

func getBindingURL(fullDashboardURL, bindingGUID string) (string, error) {
	dashboardURL, err := url.Parse(fullDashboardURL)
	if err != nil {
		return "", fmt.Errorf("invalid dashboard URL from service instance: %s", fullDashboardURL)
	}

	baseURL := fmt.Sprintf("%s://%s", dashboardURL.Scheme, dashboardURL.Host)
	return fmt.Sprintf("%s/api/bindings/%s", baseURL, bindingGUID), nil
}

type Flags struct {
	MinInstances    int
	MaxInstances    int
	CPUMinThreshold int
	CPUMaxThreshold int
}

func (p *Plugin) RunWithError(dependencies CLIDependencies, flags Flags) error {
	appGUID := dependencies.App.Guid

	// get from cloud controller
	serviceBindingsURL, err := getCCQueryURL(dependencies.APIEndpoint, appGUID, dependencies.Service.Guid)
	if err != nil {
		return err
	}

	var ccResponse struct {
		Resources []struct {
			Metadata struct {
				GUID string
			}
		}
	}

	err = dependencies.JSONClient.Do("GET", serviceBindingsURL, nil, &ccResponse)
	if err != nil {
		return fmt.Errorf("couldn't retrieve service binding: %s", err)
	}

	if len(ccResponse.Resources) != 1 {
		return fmt.Errorf("couldn't find service binding for %s to %s", dependencies.AppName, dependencies.ServiceName)
	}

	// get from autoscaling
	fullURL, err := getBindingURL(dependencies.Service.DashboardUrl, ccResponse.Resources[0].Metadata.GUID)
	if err != nil {
		return err
	}

	var autoscalingBinding AutoscalingBinding

	err = dependencies.JSONClient.Do("GET", fullURL, nil, &autoscalingBinding)
	if err != nil {
		return fmt.Errorf("autoscaling API: %s", err)
	}

	if flags.MinInstances > 0 {
		autoscalingBinding.MinInstances = flags.MinInstances
	}

	if flags.MaxInstances > 0 {
		autoscalingBinding.MaxInstances = flags.MaxInstances
	}

	if flags.CPUMinThreshold > 0 {
		autoscalingBinding.CPUMinThreshold = flags.CPUMinThreshold
	}

	if flags.CPUMaxThreshold > 0 {
		autoscalingBinding.CPUMaxThreshold = flags.CPUMaxThreshold
	}

	if autoscalingBinding.MinInstances > autoscalingBinding.MaxInstances {
		return fmt.Errorf("min instances must be <= max instances")
	}

	if autoscalingBinding.CPUMinThreshold > autoscalingBinding.CPUMaxThreshold {
		return fmt.Errorf("CPU min threshold must be <= CPU max threshold")
	}

	// autoscaling response does not include the app guid, so we have to set it
	autoscalingBinding.AppGuid = appGUID
	autoscalingBinding.Enabled = true

	// post to autoscaling
	err = dependencies.JSONClient.Do("POST", fullURL, &autoscalingBinding, nil)
	if err != nil {
		return fmt.Errorf("autoscaling API: %s", err)
	}

	return nil
}

func (p *Plugin) Run(cliConnection plugin.CliConnection, args []string) {
	logger := log.New(os.Stdout, "", 0)

	var flags Flags
	flagSet := flag.NewFlagSet("configure-autoscaling", flag.ContinueOnError)
	flagSet.IntVar(&flags.MinInstances, "min-instances", 0, "(optional) set the minimum instance count")
	flagSet.IntVar(&flags.MaxInstances, "max-instances", 0, "(optional) set the maximum instance count")
	flagSet.IntVar(&flags.CPUMinThreshold, "min-threshold", 0, "(optional) set the minimum cpu threshold percentage")
	flagSet.IntVar(&flags.CPUMaxThreshold, "max-threshold", 0, "(optional) set the maximum cpu threshold percentage")
	err := flagSet.Parse(args[1:])
	if err != nil {
		logger.Fatalf("%s", err)
	}

	dependencies, err := p.FetchCLIDependencies(cliConnection, flagSet.Args())
	if err != nil {
		logger.Fatalf("%s", err)
	}

	if err := p.RunWithError(dependencies, flags); err != nil {
		logger.Fatalf("%s", err)
	}
}

func (c *Plugin) GetMetadata() plugin.PluginMetadata {
	return plugin.PluginMetadata{
		Name: "Autoscaling",
		Version: plugin.VersionType{
			Major: 0,
			Minor: 2,
			Build: 0,
		},
		MinCliVersion: plugin.VersionType{
			Major: 6,
			Minor: 12,
			Build: 0,
		},
		Commands: []plugin.Command{
			plugin.Command{
				Name:     "configure-autoscaling",
				HelpText: "Configure an instance of the Autoscaling Service",

				// UsageDetails is optional
				// It is used to show help of usage of each command
				UsageDetails: plugin.Usage{
					Usage: "configure-autoscaling\n   cf configure-autoscaling APP_NAME SERVICE_INSTANCE",
					Options: map[string]string{
						"min-instances": "(optional) set the minimum instance count",
						"max-instances": "(optional) set the maximum instance count",
						"min-threshold": "(optional) set the minimum cpu threshold percentage",
						"max-threshold": "(optional) set the maximum cpu threshold percentage",
					},
				},
			},
		},
	}
}
