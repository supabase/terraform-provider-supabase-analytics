package provider

import (
	"analytics-terraform-provider/internal/pkg/api"
	"context"
	"net/http"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ provider.Provider              = &logflareProvider{}
	_ provider.ProviderWithFunctions = &logflareProvider{}
)

// New is a helper function to simplify provider server and testing implementation.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &logflareProvider{
			version: version,
		}
	}
}

// logflareProviderModel maps provider schema data to a Go type.
type logflareProviderModel struct {
	Host        types.String `tfsdk:"host"`
	AccessToken types.String `tfsdk:"access_token"`
}

// logflareProvider is the provider implementation.
type logflareProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// Metadata returns the provider type name.
func (p *logflareProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "logflare"
	resp.Version = p.version
}

// Schema defines the provider-level schema for configuration data.
func (p *logflareProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Interact with Logflare.",
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				Description: "URI for Logflare API. Defaults to 'https://logflare.app'.",
				Optional:    true,
			},
			"access_token": schema.StringAttribute{
				Description: "Access Token for Logflare API. May also be provided via LOGFLARE_ACCESS_TOKEN environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
		},
	}
}

func (p *logflareProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	tflog.Info(ctx, "Configuring Logflare client")

	// Retrieve provider data from configuration
	var config logflareProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If practitioner provided a configuration value for any of the
	// attributes, it must be a known value.

	if config.Host.IsNull() {
		config.Host = types.StringValue("https://logflare.app")
	}

	if config.AccessToken.IsNull() {
		config.AccessToken = types.StringValue(os.Getenv("LOGFLARE_ACCESS_TOKEN"))
	}

	if config.AccessToken.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("access_token"),
			"Unknown Logflare Access Token",
			"The provider cannot create the Logflare API client as there is an unknown configuration value for the Logflare Access Token. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the LOGFLARE_ACCESS_TOKEN environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "logflare_host", config.Host)
	ctx = tflog.SetField(ctx, "logflare_access_token", config.AccessToken)
	ctx = tflog.MaskFieldValuesWithFieldKeys(ctx, "logflare_access_token")

	tflog.Debug(ctx, "Creating Logflare client")

	// Create a new Logflare client using the configuration values
	client, err := api.NewClientWithResponses(
		config.Host.ValueString(),
		api.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			if !config.AccessToken.IsUnknown() {
				req.Header.Set("Authorization", "Bearer "+config.AccessToken.ValueString())
			}
			return nil
		}),
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Logflare API Client",
			"An unexpected error occurred when creating the Logflare API client. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"Logflare Client Error: "+err.Error(),
		)
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client

	tflog.Info(ctx, "Configured Logflare client", map[string]any{"success": true})
}

// DataSources defines the data sources implemented in the provider.
func (p *logflareProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewEndpointQueryDataSource,
	}
}

// Resources defines the resources implemented in the provider.
func (p *logflareProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewEndpointResource,
	}
}

func (p *logflareProvider) Functions(_ context.Context) []func() function.Function {
	return nil
}
