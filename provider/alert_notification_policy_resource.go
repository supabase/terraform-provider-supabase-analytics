// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/supabase/terraform-provider-supabase-analytics/internal/pkg/api"
)

var (
	_ resource.Resource                = &AlertNotificationPolicyResource{}
	_ resource.ResourceWithImportState = &AlertNotificationPolicyResource{}
)

func NewAlertNotificationPolicyResource() resource.Resource {
	return &AlertNotificationPolicyResource{}
}

type AlertNotificationPolicyResource struct {
	client *api.ClientWithResponses
}

type NotificationPolicyResourceModel struct {
	ID                    types.Int64  `tfsdk:"id"`
	Name                  types.String `tfsdk:"name"`
	Enabled               types.Bool   `tfsdk:"enabled"`
	RuleKeys              types.List   `tfsdk:"rule_keys"`
	MinSeverity           types.String `tfsdk:"min_severity"`
	Categories            types.List   `tfsdk:"categories"`
	Identifiers           types.Map    `tfsdk:"identifiers"`
	Events                types.List   `tfsdk:"events"`
	RepeatIntervalSeconds types.Int64  `tfsdk:"repeat_interval_seconds"`
	Receivers             types.List   `tfsdk:"receivers"`
}

type notificationPolicyJSON struct {
	ID                    int64             `json:"id,omitempty"`
	Name                  string            `json:"name"`
	Enabled               bool              `json:"enabled"`
	RuleKeys              []string          `json:"rule_keys"`
	MinSeverity           *string           `json:"min_severity"`
	Categories            []string          `json:"categories"`
	Identifiers           map[string]string `json:"identifiers"`
	Events                []string          `json:"events"`
	RepeatIntervalSeconds *int64            `json:"repeat_interval_seconds"`
	Receivers             []string          `json:"receivers"`
}

func (r *AlertNotificationPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alert_notification_policy"
}

func (r *AlertNotificationPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	emptyList := listdefault.StaticValue(types.ListValueMust(types.StringType, []attr.Value{}))

	resp.Schema = schema.Schema{
		MarkdownDescription: "Decides which alert issue changes Logflare sends to the control plane, and for which receivers. " +
			"The control plane (the platform, or Studio when self-hosted) fans each event out to the destinations behind the receiver names.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Notification policy identifier",
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{MarkdownDescription: "Unique name of the policy", Required: true},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the policy sends events",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"rule_keys": schema.ListAttribute{
				MarkdownDescription: "Rule key globs, e.g. `supabase.advisor.*`. Empty matches every rule.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				Default:             emptyList,
			},
			"min_severity": schema.StringAttribute{
				MarkdownDescription: "Lowest severity to send: `critical`, `major`, `warning` or `info`. Unset sends every severity.",
				Optional:            true,
			},
			"categories": schema.ListAttribute{
				MarkdownDescription: "Categories to send: `security`, `performance`, `health`. Empty sends every category.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				Default:             emptyList,
			},
			"identifiers": schema.MapAttribute{
				MarkdownDescription: "Identifiers an alert issue must have, e.g. `{ project = \"default\" }`",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				Default:             mapdefault.StaticValue(types.MapValueMust(types.StringType, map[string]attr.Value{})),
			},
			"events": schema.ListAttribute{
				MarkdownDescription: "Changes to send: `opened`, `reopened`, `repeat`, `resolved`",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				Default: listdefault.StaticValue(types.ListValueMust(types.StringType, []attr.Value{
					types.StringValue("opened"), types.StringValue("reopened"), types.StringValue("repeat"),
				})),
			},
			"repeat_interval_seconds": schema.Int64Attribute{
				MarkdownDescription: "Resend open, unacknowledged alert issues this often (at least 300). Unset sends no repeats.",
				Optional:            true,
			},
			"receivers": schema.ListAttribute{
				MarkdownDescription: "Receiver names the control plane maps to destinations, e.g. `ops-webhook`",
				ElementType:         types.StringType,
				Required:            true,
			},
		},
	}
}

func (r *AlertNotificationPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AlertNotificationPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data NotificationPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, diags := notificationPolicyModelToPayload(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.LogflareWebApiAlertNotificationPolicyControllerCreateWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
	resp.Diagnostics.Append(notificationPolicyResponse("create", httpResp, err, http.StatusCreated)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(notificationPolicyJSONToModel(ctx, httpResp.Body, &data)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AlertNotificationPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data NotificationPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.LogflareWebApiAlertNotificationPolicyControllerShowWithResponse(ctx, int(data.ID.ValueInt64()))
	if err == nil && httpResp.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(notificationPolicyResponse("read", httpResp, err, http.StatusOK)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(notificationPolicyJSONToModel(ctx, httpResp.Body, &data)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AlertNotificationPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data, state NotificationPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, diags := notificationPolicyModelToPayload(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.LogflareWebApiAlertNotificationPolicyControllerUpdateWithBodyWithResponse(
		ctx, int(state.ID.ValueInt64()), "application/json", bytes.NewReader(body),
	)
	resp.Diagnostics.Append(notificationPolicyResponse("update", httpResp, err, http.StatusOK)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(notificationPolicyJSONToModel(ctx, httpResp.Body, &data)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AlertNotificationPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data NotificationPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.LogflareWebApiAlertNotificationPolicyControllerDeleteWithResponse(ctx, int(data.ID.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete notification policy, got error: %s", err))
		return
	}

	if code := httpResp.StatusCode(); code != http.StatusNoContent && code != http.StatusNotFound {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete notification policy, got status %d: %s", code, httpResp.Body))
	}
}

// ImportState takes the numeric notification policy id.
func (r *AlertNotificationPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Import ID", fmt.Sprintf("Expected a numeric notification policy id, got %q", req.ID))
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

type statusResponse interface {
	StatusCode() int
}

func notificationPolicyResponse(op string, httpResp statusResponse, err error, want int) diag.Diagnostics {
	var diags diag.Diagnostics
	if err != nil {
		diags.AddError("Client Error", fmt.Sprintf("Unable to %s notification policy, got error: %s", op, err))
		return diags
	}

	if code := httpResp.StatusCode(); code != want {
		diags.AddError("Client Error", fmt.Sprintf("Unable to %s notification policy, got status %d", op, code))
	}
	return diags
}

func notificationPolicyModelToPayload(ctx context.Context, data *NotificationPolicyResourceModel) ([]byte, diag.Diagnostics) {
	var diags diag.Diagnostics
	payload := notificationPolicyJSON{
		Name:        data.Name.ValueString(),
		Enabled:     data.Enabled.ValueBool(),
		RuleKeys:    []string{},
		Categories:  []string{},
		Identifiers: map[string]string{},
		Events:      []string{},
		Receivers:   []string{},
	}

	diags.Append(data.RuleKeys.ElementsAs(ctx, &payload.RuleKeys, false)...)
	diags.Append(data.Categories.ElementsAs(ctx, &payload.Categories, false)...)
	diags.Append(data.Identifiers.ElementsAs(ctx, &payload.Identifiers, false)...)
	diags.Append(data.Events.ElementsAs(ctx, &payload.Events, false)...)
	diags.Append(data.Receivers.ElementsAs(ctx, &payload.Receivers, false)...)

	if !data.MinSeverity.IsNull() && !data.MinSeverity.IsUnknown() {
		payload.MinSeverity = data.MinSeverity.ValueStringPointer()
	}
	if !data.RepeatIntervalSeconds.IsNull() && !data.RepeatIntervalSeconds.IsUnknown() {
		payload.RepeatIntervalSeconds = data.RepeatIntervalSeconds.ValueInt64Pointer()
	}

	body, err := json.Marshal(payload)
	if err != nil {
		diags.AddError("Encoding Error", err.Error())
	}
	return body, diags
}

func notificationPolicyJSONToModel(ctx context.Context, body []byte, data *NotificationPolicyResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	var policy notificationPolicyJSON
	if err := json.Unmarshal(body, &policy); err != nil {
		diags.AddError("Decoding Error", fmt.Sprintf("Unable to decode notification policy: %s", err))
		return diags
	}

	list := func(values []string) types.List {
		value, d := types.ListValueFrom(ctx, types.StringType, nonNilStrings(values))
		diags.Append(d...)
		return value
	}

	identifiers, d := types.MapValueFrom(ctx, types.StringType, nonNilMap(policy.Identifiers))
	diags.Append(d...)

	data.ID = types.Int64Value(policy.ID)
	data.Name = types.StringValue(policy.Name)
	data.Enabled = types.BoolValue(policy.Enabled)
	data.RuleKeys = list(policy.RuleKeys)
	data.MinSeverity = types.StringPointerValue(policy.MinSeverity)
	data.Categories = list(policy.Categories)
	data.Identifiers = identifiers
	data.Events = list(policy.Events)
	data.RepeatIntervalSeconds = types.Int64PointerValue(policy.RepeatIntervalSeconds)
	data.Receivers = list(policy.Receivers)
	return diags
}
