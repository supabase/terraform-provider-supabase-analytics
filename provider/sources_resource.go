// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/supabase/terraform-provider-supabase-analytics/internal/pkg/api"
)

var (
	_ resource.Resource = &SourceResource{}
)

func NewSourceResource() resource.Resource {
	return &SourceResource{}
}

type SourceResource struct {
	client *api.ClientWithResponses
}

type SourceResourceModel struct {
	ApiQuota                    types.Int32          `tfsdk:"api_quota"`
	BigqueryTableTtl            types.Int32          `tfsdk:"bigquery_table_ttl"`
	BqTableId                   types.String         `tfsdk:"bq_table_id"`
	CustomEventMessageKeys      types.String         `tfsdk:"custom_event_message_keys"`
	DefaultIngestBackendEnabled types.Bool           `tfsdk:"default_ingest_backend_enabled"`
	Favorite                    types.Bool           `tfsdk:"favorite"`
	HasRejectedEvents           types.Bool           `tfsdk:"has_rejected_events"`
	Id                          types.Int64          `tfsdk:"id"`
	InsertedAt                  types.String         `tfsdk:"inserted_at"`
	Metrics                     jsontypes.Normalized `tfsdk:"metrics"`
	Name                        types.String         `tfsdk:"name"`
	Notifications               types.Object         `tfsdk:"notifications"`
	PublicToken                 types.String         `tfsdk:"public_token"`
	SlackHookUrl                types.String         `tfsdk:"slack_hook_url"`
	Token                       types.String         `tfsdk:"token"`
	UpdatedAt                   types.String         `tfsdk:"updated_at"`
	WebhookNotificationUrl      types.String         `tfsdk:"webhook_notification_url"`
}

type NotificationModel struct {
	OtherEmailNotifications       types.String `tfsdk:"other_email_notifications"`
	TeamUserIdsForEmail           types.List   `tfsdk:"team_user_ids_for_email"`
	TeamUserIdsForSchemaUpdates   types.List   `tfsdk:"team_user_ids_for_schema_updates"`
	TeamUserIdsForSms             types.List   `tfsdk:"team_user_ids_for_sms"`
	UserEmailNotifications        types.Bool   `tfsdk:"user_email_notifications"`
	UserSchemaUpdateNotifications types.Bool   `tfsdk:"user_schema_update_notifications"`
	UserTextNotifications         types.Bool   `tfsdk:"user_text_notifications"`
}

func (m NotificationModel) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"other_email_notifications":        types.StringType,
		"team_user_ids_for_email":          types.ListType{ElemType: types.StringType},
		"team_user_ids_for_schema_updates": types.ListType{ElemType: types.StringType},
		"team_user_ids_for_sms":            types.ListType{ElemType: types.StringType},
		"user_email_notifications":         types.BoolType,
		"user_schema_update_notifications": types.BoolType,
		"user_text_notifications":          types.BoolType,
	}
}

func (r *SourceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_source"
}

func (r *SourceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Source resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Endpoint identifier",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the source.",
				Required:    true,
			},
			"api_quota": schema.Int32Attribute{
				Description: "API quota for the source.",
				Optional:    true,
				Computed:    true,
				Default:     int32default.StaticInt32(25),
			},
			"bigquery_table_ttl": schema.Int32Attribute{
				Description: "BigQuery table Time-To-Live (TTL) in days.",
				Optional:    true,
				Computed:    true,
			},
			"bq_table_id": schema.StringAttribute{
				Description: "BigQuery table ID.",
				Optional:    true,
				Computed:    true,
			},
			"custom_event_message_keys": schema.StringAttribute{
				Description: "Custom event message keys.",
				Optional:    true,
				Computed:    true,
			},
			"default_ingest_backend_enabled": schema.BoolAttribute{
				Description: "Whether the default ingest backend is enabled.",
				Optional:    true,
				Computed:    true,
			},
			"favorite": schema.BoolAttribute{
				Description: "Whether the source is marked as a favorite.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"has_rejected_events": schema.BoolAttribute{
				Description: "Whether the source has rejected events.",
				Computed:    true,
			},
			"inserted_at": schema.StringAttribute{
				Description: "Timestamp of when the source was created.",
				Computed:    true,
			},
			"metrics": schema.StringAttribute{
				Description: "Metrics for the source, as a JSON string.",
				Optional:    true,
				Computed:    true,
				CustomType:  jsontypes.NormalizedType{},
			},
			"notifications": schema.ObjectAttribute{
				Description:    "Notification settings for the source.",
				Optional:       true,
				Computed:       true,
				AttributeTypes: NotificationModel{}.AttributeTypes(),
			},
			"public_token": schema.StringAttribute{
				Description: "Public token for the source.",
				Computed:    true,
				Sensitive:   true,
			},
			"slack_hook_url": schema.StringAttribute{
				Description: "Slack webhook URL for notifications.",
				Optional:    true,
				Computed:    true,
				Sensitive:   true,
			},
			"token": schema.StringAttribute{
				Description: "Private token for the source.",
				Computed:    true,
				Sensitive:   true,
			},
			"updated_at": schema.StringAttribute{
				Description: "Timestamp of when the source was last updated.",
				Computed:    true,
			},
			"webhook_notification_url": schema.StringAttribute{
				Description: "Webhook URL for notifications.",
				Optional:    true,
				Computed:    true,
				Sensitive:   true,
			},
		},
	}
}

func (r *SourceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *SourceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SourceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(createSource(ctx, &data, r.client)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func createSource(ctx context.Context, data *SourceResourceModel, client *api.ClientWithResponses) diag.Diagnostics {
	body, diags := sourceModelToApiSchema(ctx, data)
	if diags.HasError() {
		return diags
	}

	httpResp, err := client.LogflareWebApiSourceControllerCreateWithResponse(ctx, body)
	if err != nil {
		msg := fmt.Sprintf("Unable to create endpoint, got error: %s", err)
		return diag.Diagnostics{diag.NewErrorDiagnostic("Client Error", msg)}
	}

	if httpResp.StatusCode() != 201 {
		msg := fmt.Sprintf("Unable to create endpoint, got status %d: %s", httpResp.StatusCode(), httpResp.Body)
		return diag.Diagnostics{diag.NewErrorDiagnostic("Client Error", msg)}
	}

	return sourceSchemaToModel(ctx, httpResp.JSON201, data)
}

func (r *SourceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SourceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Token.IsNull() {
		resp.Diagnostics.AddWarning("Resource Read Ignored", "Source token is null, cannot read.")
		return
	}

	resp.Diagnostics.Append(readSource(ctx, &data, r.client)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func readSource(ctx context.Context, data *SourceResourceModel, client *api.ClientWithResponses) diag.Diagnostics {
	httpResp, err := client.LogflareWebApiSourceControllerShowWithResponse(ctx, data.Token.ValueString())
	if err != nil {
		msg := fmt.Sprintf("Unable to read source, got error: %s", err)
		return diag.Diagnostics{diag.NewErrorDiagnostic("Client Error", msg)}
	}

	if httpResp.StatusCode() != 200 {
		msg := fmt.Sprintf("Unable to read source, got status %d: %s", httpResp.StatusCode(), httpResp.Body)
		return diag.Diagnostics{diag.NewErrorDiagnostic("Client Error", msg)}
	}

	var result = httpResp.JSON200
	return sourceSchemaToModel(ctx, result, data)
}

func (r *SourceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data SourceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(updateSource(ctx, &data, r.client)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func updateSource(ctx context.Context, data *SourceResourceModel, client *api.ClientWithResponses) diag.Diagnostics {
	body, diags := sourceModelToApiSchema(ctx, data)
	if diags.HasError() {
		return diags
	}

	httpResp, err := client.LogflareWebApiSourceControllerUpdateWithResponse(ctx, data.Token.ValueString(), body)
	if err != nil {
		msg := fmt.Sprintf("Unable to update source, got error: %s", err)
		return diag.Diagnostics{diag.NewErrorDiagnostic("Client Error", msg)}
	}

	if httpResp.StatusCode() < 200 || httpResp.StatusCode() >= 300 {
		msg := fmt.Sprintf("Unable to update source, got status %d: %s", httpResp.StatusCode(), httpResp.Body)
		return diag.Diagnostics{diag.NewErrorDiagnostic("Client Error", msg)}
	}

	return readSource(ctx, data, client)
}

func (r *SourceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SourceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Token.IsNull() {
		return
	}

	resp.Diagnostics.Append(deleteSource(ctx, &data, r.client)...)
}

func deleteSource(ctx context.Context, data *SourceResourceModel, client *api.ClientWithResponses) diag.Diagnostics {
	httpResp, err := client.LogflareWebApiSourceControllerDeleteWithResponse(ctx, data.Token.ValueString())
	if err != nil {
		msg := fmt.Sprintf("Unable to delete source, got error: %s", err)
		return diag.Diagnostics{diag.NewErrorDiagnostic("Client Error", msg)}
	}

	if httpResp.HTTPResponse.StatusCode != 204 {
		msg := fmt.Sprintf("Unable to delete source, got status %d: %s", httpResp.StatusCode(), httpResp.Body)
		return diag.Diagnostics{diag.NewErrorDiagnostic("Client Error", msg)}
	}

	return nil
}

func sourceSchemaToModel(ctx context.Context, result *api.Source, data *SourceResourceModel) diag.Diagnostics {
	data.Id = types.Int64Value(int64(*result.Id))
	data.Name = types.StringValue(result.Name)
	data.ApiQuota = types.Int32PointerValue(intPtrToInt32Ptr(result.ApiQuota))
	data.BigqueryTableTtl = types.Int32PointerValue(intPtrToInt32Ptr(result.BigqueryTableTtl))
	data.BqTableId = types.StringPointerValue(result.BqTableId)
	data.CustomEventMessageKeys = types.StringPointerValue(result.CustomEventMessageKeys)
	data.DefaultIngestBackendEnabled = types.BoolPointerValue(result.DefaultIngestBackendEnabled)
	data.Favorite = types.BoolPointerValue(result.Favorite)
	data.HasRejectedEvents = types.BoolPointerValue(result.HasRejectedEvents)
	data.PublicToken = types.StringPointerValue(result.PublicToken)
	data.SlackHookUrl = types.StringPointerValue(result.SlackHookUrl)
	data.Token = types.StringPointerValue(result.Token)
	data.WebhookNotificationUrl = types.StringPointerValue(result.WebhookNotificationUrl)

	if result.InsertedAt == nil {
		data.InsertedAt = types.StringNull()
	} else {
		data.InsertedAt = types.StringValue(result.InsertedAt.Format(time.RFC3339))
	}

	if result.UpdatedAt == nil {
		data.UpdatedAt = types.StringNull()
	} else {
		data.UpdatedAt = types.StringValue(result.UpdatedAt.Format(time.RFC3339))
	}

	if result.Metrics != nil {
		value, err := json.Marshal(result.Metrics)
		if err != nil {
			return diag.Diagnostics{diag.NewErrorDiagnostic("Can't encode 'metrics' field", err.Error())}
		}
		data.Metrics = jsontypes.NewNormalizedValue(string(value))
	} else {
		data.Metrics = jsontypes.NewNormalizedValue("{}")
	}

	if result.Notifications != nil {
		var diags, listDiags diag.Diagnostics
		var apiNotifications api.Notification

		b, err := json.Marshal(result.Notifications)
		if err != nil {
			return diag.Diagnostics{diag.NewErrorDiagnostic("Can't marshal 'notifications' map", err.Error())}
		}
		err = json.Unmarshal(b, &apiNotifications)
		if err != nil {
			return diag.Diagnostics{diag.NewErrorDiagnostic("Can't unmarshal 'notifications' map into struct", err.Error())}
		}

		model := NotificationModel{
			OtherEmailNotifications:       types.StringPointerValue(apiNotifications.OtherEmailNotifications),
			UserEmailNotifications:        types.BoolPointerValue(apiNotifications.UserEmailNotifications),
			UserSchemaUpdateNotifications: types.BoolPointerValue(apiNotifications.UserSchemaUpdateNotifications),
			UserTextNotifications:         types.BoolPointerValue(apiNotifications.UserTextNotifications),
		}

		if apiNotifications.TeamUserIdsForEmail != nil {
			model.TeamUserIdsForEmail, diags = types.ListValueFrom(ctx, types.StringType, *apiNotifications.TeamUserIdsForEmail)
			listDiags.Append(diags...)
		} else {
			model.TeamUserIdsForEmail, diags = types.ListValue(types.StringType, nil)
			listDiags.Append(diags...)
		}

		if apiNotifications.TeamUserIdsForSchemaUpdates != nil {
			model.TeamUserIdsForSchemaUpdates, diags = types.ListValueFrom(ctx, types.StringType, *apiNotifications.TeamUserIdsForSchemaUpdates)
			listDiags.Append(diags...)
		} else {
			model.TeamUserIdsForSchemaUpdates, diags = types.ListValue(types.StringType, nil)
			listDiags.Append(diags...)
		}

		if apiNotifications.TeamUserIdsForSms != nil {
			model.TeamUserIdsForSms, diags = types.ListValueFrom(ctx, types.StringType, *apiNotifications.TeamUserIdsForSms)
			listDiags.Append(diags...)
		} else {
			model.TeamUserIdsForSms, diags = types.ListValue(types.StringType, nil)
			listDiags.Append(diags...)
		}

		if listDiags.HasError() {
			return listDiags
		}

		data.Notifications, diags = types.ObjectValueFrom(ctx, NotificationModel{}.AttributeTypes(), &model)
		if diags.HasError() {
			return diags
		}

	} else {
		data.Notifications = types.ObjectNull(NotificationModel{}.AttributeTypes())
	}

	return nil
}

func sourceModelToApiSchema(ctx context.Context, data *SourceResourceModel) (api.Source, diag.Diagnostics) {
	var metrics *map[string]any
	data.Metrics.Unmarshal(&metrics)
	var diags, modelDiags diag.Diagnostics

	body := api.Source{
		Name:                        data.Name.ValueString(),
		ApiQuota:                    int32PtrToIntPtr(data.ApiQuota.ValueInt32Pointer()),
		BigqueryTableTtl:            int32PtrToIntPtr(data.BigqueryTableTtl.ValueInt32Pointer()),
		BqTableId:                   data.BqTableId.ValueStringPointer(),
		CustomEventMessageKeys:      data.CustomEventMessageKeys.ValueStringPointer(),
		DefaultIngestBackendEnabled: data.DefaultIngestBackendEnabled.ValueBoolPointer(),
		Favorite:                    data.Favorite.ValueBoolPointer(),
		Metrics:                     metrics,
		SlackHookUrl:                data.SlackHookUrl.ValueStringPointer(),
		Token:                       data.Token.ValueStringPointer(),
		WebhookNotificationUrl:      data.WebhookNotificationUrl.ValueStringPointer(),
	}

	if !data.Notifications.IsNull() && !data.Notifications.IsUnknown() {
		var model NotificationModel
		diags = data.Notifications.As(ctx, &model, basetypes.ObjectAsOptions{})
		if diags.HasError() {
			return body, diags
		}

		apiNotifications := api.Notification{
			OtherEmailNotifications:       model.OtherEmailNotifications.ValueStringPointer(),
			UserEmailNotifications:        model.UserEmailNotifications.ValueBoolPointer(),
			UserSchemaUpdateNotifications: model.UserSchemaUpdateNotifications.ValueBoolPointer(),
			UserTextNotifications:         model.UserTextNotifications.ValueBoolPointer(),
		}

		if !model.TeamUserIdsForEmail.IsNull() {
			var teamUserIdsForEmail []string
			diags = model.TeamUserIdsForEmail.ElementsAs(ctx, &teamUserIdsForEmail, false)
			modelDiags.Append(diags...)
			apiNotifications.TeamUserIdsForEmail = &teamUserIdsForEmail
		}

		if !model.TeamUserIdsForSchemaUpdates.IsNull() {
			var teamUserIdsForSchemaUpdates []string
			diags = model.TeamUserIdsForSchemaUpdates.ElementsAs(ctx, &teamUserIdsForSchemaUpdates, false)
			modelDiags.Append(diags...)
			apiNotifications.TeamUserIdsForSchemaUpdates = &teamUserIdsForSchemaUpdates
		}

		if !model.TeamUserIdsForSms.IsNull() {
			var teamUserIdsForSms []string
			diags = model.TeamUserIdsForSms.ElementsAs(ctx, &teamUserIdsForSms, false)
			modelDiags.Append(diags...)
			apiNotifications.TeamUserIdsForSms = &teamUserIdsForSms
		}

		if modelDiags.HasError() {
			return body, modelDiags
		}

		var notificationsMap map[string]interface{}
		b, err := json.Marshal(apiNotifications)
		if err != nil {
			diags.AddError("Can't marshal 'notifications' struct", err.Error())
			return body, diags
		}
		err = json.Unmarshal(b, &notificationsMap)
		if err != nil {
			diags.AddError("Can't unmarshal 'notifications' to map", err.Error())
			return body, diags
		}
		body.Notifications = &notificationsMap
	}

	return body, diags
}
