package mocks

import "code.cloudfoundry.org/cli/plugin/models"

type CLIConnection struct {
	IsLoggedInCall struct {
		Returns struct {
			LoggedIn bool
			Error    error
		}
	}
	AccessTokenCall struct {
		Returns struct {
			Token string
			Error error
		}
	}

	GetServiceCall struct {
		Receives struct {
			ServiceName string
		}

		Returns struct {
			Service plugin_models.GetService_Model
			Error   error
		}
	}

	GetAppCall struct {
		Receives struct {
			AppName string
		}

		Returns struct {
			App   plugin_models.GetAppModel
			Error error
		}
	}

	ApiEndpointCall struct {
		Returns struct {
			ApiEndpoint string
			Error       error
		}
	}

	IsSSLDisabledCall struct {
		Returns struct {
			Disabled bool
			Error    error
		}
	}
}

func (c *CLIConnection) IsLoggedIn() (bool, error) {
	return c.IsLoggedInCall.Returns.LoggedIn, c.IsLoggedInCall.Returns.Error
}

func (c *CLIConnection) AccessToken() (string, error) {
	return c.AccessTokenCall.Returns.Token, c.AccessTokenCall.Returns.Error
}

func (c *CLIConnection) GetService(name string) (plugin_models.GetService_Model, error) {
	c.GetServiceCall.Receives.ServiceName = name

	return c.GetServiceCall.Returns.Service, c.GetServiceCall.Returns.Error
}

func (c *CLIConnection) ApiEndpoint() (string, error) {
	return c.ApiEndpointCall.Returns.ApiEndpoint, c.ApiEndpointCall.Returns.Error
}

func (c *CLIConnection) GetApp(name string) (plugin_models.GetAppModel, error) {
	c.GetAppCall.Receives.AppName = name

	return c.GetAppCall.Returns.App, c.GetAppCall.Returns.Error
}

func (c *CLIConnection) IsSSLDisabled() (bool, error) {
	return c.IsSSLDisabledCall.Returns.Disabled, c.IsSSLDisabledCall.Returns.Error
}
