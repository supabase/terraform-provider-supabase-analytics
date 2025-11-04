// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/supabase/terraform-provider-supabase-analytics/internal/pkg/api"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ datasource.DataSource              = &EndpointQueryDataSource{}
	_ datasource.DataSourceWithConfigure = &EndpointQueryDataSource{}
)

func NewEndpointQueryDataSource() datasource.DataSource {
	return &EndpointQueryDataSource{}
}

// ExampleDataSource defines the data source implementation.
type EndpointQueryDataSource struct {
	client *api.ClientWithResponses
}

// ExampleDataSourceModel describes the data source data model.
type EndpointQueryDataSourceModel = struct {
	NameOrToken types.String  `tfsdk:"name_or_token"`
	Result      types.Dynamic `tfsdk:"result"`
}

func (d *EndpointQueryDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_endpoint_query"
}

func (d *EndpointQueryDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
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

func (d *EndpointQueryDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *EndpointQueryDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EndpointQueryDataSourceModel

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

func readEndpoints(ctx context.Context, data *EndpointQueryDataSourceModel, client *api.ClientWithResponses) diag.Diagnostics {
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

	// Convert the API response to a list of dynamic values
	resultList := *httpResp.JSON200.Result
	dynamicValues := make([]attr.Value, 0, len(resultList))

	for _, item := range resultList {
		objValue, diags := convertMapToObject(item)
		if diags.HasError() {
			return diags
		}

		dynamicValue := types.DynamicValue(objValue)
		dynamicValues = append(dynamicValues, dynamicValue)
	}

	listValue, diags := types.ListValue(types.DynamicType, dynamicValues)
	if diags.HasError() {
		return diags
	}

	data.Result = types.DynamicValue(listValue)

	return nil
}

func convertMapToObject(m map[string]any) (basetypes.ObjectValue, diag.Diagnostics) {
	attrTypes := make(map[string]attr.Type)
	attrValues := make(map[string]attr.Value)

	for key, value := range m {
		convertedValue, valueType := convertInterfaceToValue(value)
		attrTypes[key] = valueType
		attrValues[key] = convertedValue
	}

	return types.ObjectValue(attrTypes, attrValues)
}

func convertInterfaceToValue(value any) (attr.Value, attr.Type) {
	if value == nil {
		return types.StringNull(), types.StringType
	}

	switch v := value.(type) {
	case string:
		return types.StringValue(v), types.StringType
	case float64:
		return types.Float64Value(v), types.Float64Type
	case int:
		return types.Int64Value(int64(v)), types.Int64Type
	case int64:
		return types.Int64Value(v), types.Int64Type
	case bool:
		return types.BoolValue(v), types.BoolType
	case []any:
		// Handle nested arrays
		elements := make([]attr.Value, len(v))
		if len(v) == 0 {
			// Empty list defaults to string type
			listVal, _ := types.ListValue(types.StringType, elements)
			return listVal, types.ListType{ElemType: types.StringType}
		}

		var elemType attr.Type = types.StringType
		for i, item := range v {
			elements[i], elemType = convertInterfaceToValue(item)
		}
		listVal, _ := types.ListValue(elemType, elements)
		return listVal, types.ListType{ElemType: elemType}
	case map[string]any:
		// Handle nested objects
		objVal, _ := convertMapToObject(v)
		return objVal, objVal.Type(context.Background())
	default:
		// Fallback to string representation
		return types.StringValue(fmt.Sprintf("%v", v)), types.StringType
	}
}
