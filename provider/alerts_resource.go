// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/supabase/terraform-provider-supabase-analytics/internal/pkg/api"
)

var (
	_ resource.Resource                = &AlertResource{}
	_ resource.ResourceWithImportState = &AlertResource{}
)

func NewAlertResource() resource.Resource {
	return &AlertResource{}
}

type AlertResource struct {
	client *api.ClientWithResponses
}

func (r *AlertResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alert"
}

func (r *AlertResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	optionalString := func(description string) schema.StringAttribute {
		return schema.StringAttribute{MarkdownDescription: description, Optional: true}
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Logflare alert. Setting `rule_key` makes it an alert issue rule: each run turns result rows into alert issues.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Alert identifier",
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"token": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Alert token used by the Logflare API",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name":        schema.StringAttribute{MarkdownDescription: "Name of the alert", Required: true},
			"description": optionalString("Description of the alert"),
			"language": schema.StringAttribute{
				MarkdownDescription: "Query language: `bq_sql`, `pg_sql` or `lql`",
				Required:            true,
			},
			"query": schema.StringAttribute{MarkdownDescription: "Query run on every schedule tick", Required: true},
			"cron":  schema.StringAttribute{MarkdownDescription: "Cron schedule", Required: true},
			"rule_key": optionalString(
				"Stable key of the alert issue rule, e.g. `supabase.edge-function-5xx`. Makes this alert an alert issue rule.",
			),
			"managed": schema.BoolAttribute{
				MarkdownDescription: "Whether the rule is a managed preset",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"severity": schema.StringAttribute{
				MarkdownDescription: "Alert issue severity: `critical`, `major`, `warning` or `info`",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("warning"),
			},
			"category": schema.StringAttribute{
				MarkdownDescription: "Alert issue category: `security`, `performance` or `health`",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("health"),
			},
			"group_by": schema.ListAttribute{
				MarkdownDescription: "Result columns that split rows into separate alert issues",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				Default:             listdefault.StaticValue(types.ListValueMust(types.StringType, []attr.Value{})),
			},
			"scope": schema.MapAttribute{
				MarkdownDescription: "Literal identifiers added to every alert issue, e.g. `{ project = \"default\" }`",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				Default:             mapdefault.StaticValue(types.MapValueMust(types.StringType, map[string]attr.Value{})),
			},
			"resolve_after_runs": schema.Int64Attribute{
				MarkdownDescription: "Clean runs before an alert issue resolves",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(2),
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the alert runs",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"signal_mapping": schema.SingleNestedAttribute{
				MarkdownDescription: "How result rows become alert issue signals",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"identifiers": schema.MapAttribute{
						MarkdownDescription: "Identifier key to result column",
						ElementType:         types.StringType,
						Optional:            true,
					},
					"title_column":       optionalString("Column used as the alert issue title"),
					"description_column": optionalString("Column used as the alert issue description"),
					"severity_column":    optionalString("Column that overrides the severity"),
					"category_column":    optionalString("Column that overrides the category"),
					"metadata_columns": schema.ListAttribute{
						MarkdownDescription: "Columns copied into the alert issue metadata",
						ElementType:         types.StringType,
						Optional:            true,
					},
					"window_seconds": schema.Int64Attribute{
						MarkdownDescription: "Look-back window for `lql` rules",
						Optional:            true,
					},
					"preview": schema.SingleNestedAttribute{
						MarkdownDescription: "Log preview shown on each alert issue",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"lql": optionalString("LQL with `f:<source>` and `{{identifier}}` placeholders"),
						},
					},
					"notebook": schema.ListNestedAttribute{
						MarkdownDescription: "SQL templates shown as notebook cells on each alert issue",
						Optional:            true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"title":  schema.StringAttribute{Required: true, MarkdownDescription: "Cell title"},
								"source": schema.StringAttribute{Required: true, MarkdownDescription: "`database` or `logs`"},
								"sql": schema.StringAttribute{
									Required:            true,
									MarkdownDescription: "SQL with `{{ident:key}}` and `{{literal:key}}` placeholders",
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *AlertResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AlertResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AlertResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, diags := alertModelToPayload(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.LogflareWebApiAlertControllerCreateWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
	resp.Diagnostics.Append(alertResponseToModel(ctx, "create", httpResp, err, http.StatusCreated, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AlertResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AlertResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() || data.Token.IsNull() {
		return
	}

	httpResp, err := r.client.LogflareWebApiAlertControllerShowWithResponse(ctx, data.Token.ValueString())
	if err == nil && httpResp.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(alertResponseToModel(ctx, "read", httpResp, err, http.StatusOK, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AlertResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AlertResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, diags := alertModelToPayload(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.LogflareWebApiAlertControllerUpdateWithBodyWithResponse(
		ctx, data.Token.ValueString(), "application/json", bytes.NewReader(body),
	)
	resp.Diagnostics.Append(alertResponseToModel(ctx, "update", httpResp, err, http.StatusOK, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AlertResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AlertResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() || data.Token.IsNull() {
		return
	}

	httpResp, err := r.client.LogflareWebApiAlertControllerDeleteWithResponse(ctx, data.Token.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete alert, got error: %s", err))
		return
	}

	if code := httpResp.StatusCode(); code != http.StatusNoContent && code != http.StatusNotFound {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete alert, got status %d: %s", code, httpResp.Body))
	}
}

// ImportState accepts either the numeric alert id or the alert token.
func (r *AlertResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("token"), req.ID)...)
		return
	}

	httpResp, err := r.client.LogflareWebApiAlertControllerIndexWithResponse(ctx)
	if err != nil || httpResp.JSON200 == nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list alerts for import: %v", err))
		return
	}

	for _, alert := range *httpResp.JSON200 {
		if alert.Id != nil && int64(*alert.Id) == id && alert.Token != nil {
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("token"), *alert.Token)...)
			return
		}
	}

	resp.Diagnostics.AddError("Alert Not Found", fmt.Sprintf("No alert with id %d", id))
}
