// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"analytics-terraform-provider/internal/pkg/api"
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ datasource.DataSource              = &EndpointsDataSource{}
	_ datasource.DataSourceWithConfigure = &EndpointsDataSource{}
)

func NewEndpointsDataSource() datasource.DataSource {
	return &EndpointsDataSource{}
}

// ExampleDataSource defines the data source implementation.
type EndpointsDataSource struct {
	client *api.ClientWithResponses
}

// ExampleDataSourceModel describes the data source data model.
type EndpointsDataSourceModel = struct {
	NameOrToken types.String  `tfsdk:"name_or_token"`
	Result      types.Dynamic `tfsdk:"result"`
}

func (d *EndpointsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_endpoints"
}

func (d *EndpointsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Logflare Endpoint Data source",

		Attributes: map[string]schema.Attribute{
			"result": schema.DynamicAttribute{
				MarkdownDescription: "A list of results for your query endpoint.",
				Computed:            true,
			},
			"name_or_token": schema.StringAttribute{
				MarkdownDescription: "Logflare access token",
				Required:            true,
				Sensitive:           true,
			},
		},
	}
}

func (d *EndpointsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*api.ClientWithResponses)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected **api.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.client = client
}

func (d *EndpointsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EndpointsDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(readEndpoints(ctx, &data, d.client)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func readEndpoints(ctx context.Context, data *EndpointsDataSourceModel, client *api.ClientWithResponses) diag.Diagnostics {
	httpResp, err := client.LogflareWebEndpointsControllerQuery2WithResponse(ctx, data.NameOrToken.ValueString())
	if err != nil {
		msg := fmt.Sprintf("Unable to read endpoints, got error: %s", err)
		return diag.Diagnostics{diag.NewErrorDiagnostic("Client Error", msg)}
	}

	if httpResp.JSON200 == nil {
		msg := fmt.Sprintf("Unable to read endpoints, got status %d: %s", httpResp.StatusCode(), httpResp.Body)
		return diag.Diagnostics{diag.NewErrorDiagnostic("Client Error", msg)}
	}

	if httpResp.JSON200.Error != nil {
		msg := fmt.Sprintf("Endpoints API returned an error: %s", httpResp.JSON200.Error)
		return diag.Diagnostics{diag.NewErrorDiagnostic("Response Error", msg)}
	}

	listVal, diags := types.ListValueFrom(ctx, types.DynamicType, *httpResp.JSON200.Result)
	if diags.HasError() {
		return diags
	}

	data.Result = types.DynamicValue(listVal)

	return diags
}
