// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"analytics-terraform-provider/internal/pkg/api"
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &EndpointResource{}
	_ resource.ResourceWithImportState = &EndpointResource{}
)

func NewEndpointResource() resource.Resource {
	return &EndpointResource{}
}

// EndpointResource defines the resource implementation.
type EndpointResource struct {
	client *api.ClientWithResponses
}

// EndpointResourceModel describes the resource data model.
type EndpointResourceModel struct {
	CacheDurationSeconds       types.Int32          `tfsdk:"cache_duration_seconds"`
	Description                types.String         `tfsdk:"description"`
	EnableAuth                 types.Bool           `tfsdk:"enable_auth"`
	Id                         types.Int64          `tfsdk:"id"`
	MaxLimit                   types.Int32          `tfsdk:"max_limit"`
	Name                       types.String         `tfsdk:"name"`
	ProactiveRequeryingSeconds types.Int32          `tfsdk:"proactive_requerying_seconds"`
	Query                      types.String         `tfsdk:"query"`
	Sandboxable                types.Bool           `tfsdk:"sandboxable"`
	SourceMapping              jsontypes.Normalized `tfsdk:"source_mapping"`
	Token                      types.String         `tfsdk:"token"`
}

func (r *EndpointResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_endpoint"
}

func (r *EndpointResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Endpoint resource",

		Attributes: map[string]schema.Attribute{
			"cache_duration_seconds": schema.Int32Attribute{
				MarkdownDescription: "Cache duration in seconds",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(3600),
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the endpoint",
				Optional:            true,
			},
			"enable_auth": schema.BoolAttribute{
				MarkdownDescription: "Enable authentication for the endpoint",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Endpoint identifier",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"max_limit": schema.Int32Attribute{
				MarkdownDescription: "Maximum limit",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(1000),
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the endpoint",
				Required:            true,
			},
			"proactive_requerying_seconds": schema.Int32Attribute{
				MarkdownDescription: "Proactive requerying interval in seconds",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(1800),
			},
			"query": schema.StringAttribute{
				MarkdownDescription: "Query string",
				Required:            true,
			},
			"sandboxable": schema.BoolAttribute{
				MarkdownDescription: "Whether the endpoint is sandboxable",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"source_mapping": schema.StringAttribute{
				CustomType:          jsontypes.NormalizedType{},
				MarkdownDescription: "Source mapping as JSON",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("{}"),
			},
			"token": schema.StringAttribute{
				MarkdownDescription: "Authentication token",
				Computed:            true,
				Sensitive:           true,
			},
		},
	}
}

func (r *EndpointResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*api.ClientWithResponses)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *api.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *EndpointResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data EndpointResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(createEndpoint(ctx, &data, r.client)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "created a resource")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func createEndpoint(ctx context.Context, data *EndpointResourceModel, client *api.ClientWithResponses) diag.Diagnostics {
	var body = endpointResourcetoApiSchema(data)
	httpResp, err := client.LogflareWebApiEndpointControllerCreateWithResponse(ctx, body)
	if err != nil {
		msg := fmt.Sprintf("Unable to create endpoint, got error: %s", err)
		return diag.Diagnostics{diag.NewErrorDiagnostic("Client Error", msg)}
	}

	if httpResp.JSON201 == nil {
		msg := fmt.Sprintf("Unable to create endpoint, got status %d: %s", httpResp.StatusCode(), httpResp.Body)
		return diag.Diagnostics{diag.NewErrorDiagnostic("Client Error", msg)}
	}

	// data.Id = types.Int64Value(int64(*httpResp.JSON201.Id))

	return endpointApiSchemaToModel(httpResp.JSON201, data)
}

func (r *EndpointResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data EndpointResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Token.IsNull() {
		return
	}

	resp.Diagnostics.Append(readEndpoint(ctx, &data, r.client)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func readEndpoint(ctx context.Context, data *EndpointResourceModel, client *api.ClientWithResponses) diag.Diagnostics {
	httpResp, err := client.LogflareWebApiEndpointControllerShowWithResponse(ctx, data.Token.ValueString())
	if err != nil {
		msg := fmt.Sprintf("Unable to read endpoint, got error: %s", err)
		return diag.Diagnostics{diag.NewErrorDiagnostic("Client Error", msg)}
	}

	if httpResp.JSON200 == nil {
		msg := fmt.Sprintf("Unable to read endpoint, got status %d: %s", httpResp.StatusCode(), httpResp.Body)
		return diag.Diagnostics{diag.NewErrorDiagnostic("Client Error", msg)}
	}

	var result = httpResp.JSON200

	return endpointApiSchemaToModel(result, data)
}

func (r *EndpointResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data EndpointResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(updateEndpoint(ctx, &data, r.client)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func updateEndpoint(ctx context.Context, data *EndpointResourceModel, client *api.ClientWithResponses) diag.Diagnostics {
	var body = endpointResourcetoApiSchema(data)
	httpResp, err := client.LogflareWebApiEndpointControllerUpdateWithResponse(ctx, data.Token.ValueString(), body)
	if err != nil {
		msg := fmt.Sprintf("Unable to update endpoint, got error: %s", err)
		return diag.Diagnostics{diag.NewErrorDiagnostic("Client Error", msg)}
	}

	if httpResp.JSON200 == nil {
		msg := fmt.Sprintf("Unable to update endpoint, got status %d: %s", httpResp.StatusCode(), httpResp.Body)
		return diag.Diagnostics{diag.NewErrorDiagnostic("Client Error", msg)}
	}

	var result = httpResp.JSON200

	return endpointApiSchemaToModel(result, data)
}

func (r *EndpointResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {

	var data EndpointResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Token.IsNull() {
		return
	}

	resp.Diagnostics.Append(deleteEndpoint(ctx, &data, r.client)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func deleteEndpoint(ctx context.Context, data *EndpointResourceModel, client *api.ClientWithResponses) diag.Diagnostics {
	httpResp, err := client.LogflareWebApiEndpointControllerDeleteWithResponse(ctx, data.Token.ValueString())
	if err != nil {
		msg := fmt.Sprintf("Unable to delete endpoint, got error: %s", err)
		return diag.Diagnostics{diag.NewErrorDiagnostic("Client Error", msg)}
	}

	if httpResp.HTTPResponse.StatusCode != 204 {
		msg := fmt.Sprintf("Unable to delete endpoint, got status %d: %s", httpResp.StatusCode(), httpResp.Body)
		return diag.Diagnostics{diag.NewErrorDiagnostic("Client Error", msg)}
	}

	return nil
}

func (r *EndpointResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func int32PtrToIntPtr(i *int32) *int {
	if i == nil {
		return nil
	}
	val := int(*i)
	return &val
}

func intPtrToInt32Ptr(i *int) *int32 {
	if i == nil {
		return nil
	}
	val := int32(*i)
	return &val
}

func endpointApiSchemaToModel(result *api.EndpointApiSchema, data *EndpointResourceModel) diag.Diagnostics {
	data.Id = types.Int64Value(int64(*result.Id))
	data.CacheDurationSeconds = types.Int32PointerValue(intPtrToInt32Ptr(result.CacheDurationSeconds))
	data.Description = types.StringPointerValue(result.Description)
	data.EnableAuth = types.BoolPointerValue(result.EnableAuth)
	data.MaxLimit = types.Int32PointerValue(intPtrToInt32Ptr(result.MaxLimit))
	data.Name = types.StringValue(result.Name)
	data.ProactiveRequeryingSeconds = types.Int32PointerValue(intPtrToInt32Ptr(result.ProactiveRequeryingSeconds))
	data.Query = types.StringValue(result.Query)
	data.Sandboxable = types.BoolPointerValue(result.Sandboxable)
	value, err := json.Marshal(result.SourceMapping)
	if err != nil {
		return diag.Diagnostics{diag.NewErrorDiagnostic("Can't encode sandboxable field", err.Error())}
	}
	data.SourceMapping = jsontypes.NewNormalizedValue(string(value))
	data.Token = types.StringPointerValue(result.Token)

	return nil
}

func endpointResourcetoApiSchema(data *EndpointResourceModel) api.EndpointApiSchema {
	var source_mapping *map[string]any
	data.SourceMapping.Unmarshal(&source_mapping)
	body := api.EndpointApiSchema{
		CacheDurationSeconds:       int32PtrToIntPtr(data.CacheDurationSeconds.ValueInt32Pointer()),
		Description:                data.Description.ValueStringPointer(),
		EnableAuth:                 data.EnableAuth.ValueBoolPointer(),
		MaxLimit:                   int32PtrToIntPtr(data.MaxLimit.ValueInt32Pointer()),
		Name:                       data.Name.ValueString(),
		ProactiveRequeryingSeconds: int32PtrToIntPtr(data.ProactiveRequeryingSeconds.ValueInt32Pointer()),
		Query:                      data.Query.ValueString(),
		Sandboxable:                data.Sandboxable.ValueBoolPointer(),
		SourceMapping:              source_mapping,
		Token:                      data.Token.ValueStringPointer(),
	}

	return body
}
