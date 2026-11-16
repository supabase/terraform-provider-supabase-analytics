// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/supabase/terraform-provider-supabase-analytics/internal/pkg/api"
)

type AlertResourceModel struct {
	Category         types.String             `tfsdk:"category"`
	Cron             types.String             `tfsdk:"cron"`
	Description      types.String             `tfsdk:"description"`
	Enabled          types.Bool               `tfsdk:"enabled"`
	GroupBy          types.List               `tfsdk:"group_by"`
	Id               types.Int64              `tfsdk:"id"`
	Language         types.String             `tfsdk:"language"`
	Managed          types.Bool               `tfsdk:"managed"`
	Name             types.String             `tfsdk:"name"`
	Query            types.String             `tfsdk:"query"`
	ResolveAfterRuns types.Int64              `tfsdk:"resolve_after_runs"`
	RuleKey          types.String             `tfsdk:"rule_key"`
	Scope            types.Map                `tfsdk:"scope"`
	Severity         types.String             `tfsdk:"severity"`
	SignalMapping    *AlertSignalMappingModel `tfsdk:"signal_mapping"`
	Token            types.String             `tfsdk:"token"`
}

type AlertSignalMappingModel struct {
	CategoryColumn    types.String             `tfsdk:"category_column"`
	DescriptionColumn types.String             `tfsdk:"description_column"`
	Identifiers       types.Map                `tfsdk:"identifiers"`
	MetadataColumns   types.List               `tfsdk:"metadata_columns"`
	Notebook          []AlertNotebookCellModel `tfsdk:"notebook"`
	Preview           *AlertPreviewModel       `tfsdk:"preview"`
	SeverityColumn    types.String             `tfsdk:"severity_column"`
	TitleColumn       types.String             `tfsdk:"title_column"`
	WindowSeconds     types.Int64              `tfsdk:"window_seconds"`
}

type AlertPreviewModel struct {
	Lql types.String `tfsdk:"lql"`
}

type AlertNotebookCellModel struct {
	Source types.String `tfsdk:"source"`
	Sql    types.String `tfsdk:"sql"`
	Title  types.String `tfsdk:"title"`
}

// alertPayload is the JSON body for create and update; every field is sent so that
// removing an attribute from the configuration resets it in Logflare.
type alertPayload struct {
	Category         string            `json:"category"`
	Cron             string            `json:"cron"`
	Description      *string           `json:"description"`
	Enabled          bool              `json:"enabled"`
	GroupBy          []string          `json:"group_by"`
	Language         string            `json:"language"`
	Managed          bool              `json:"managed"`
	Name             string            `json:"name"`
	Query            string            `json:"query"`
	ResolveAfterRuns int64             `json:"resolve_after_runs"`
	RuleKey          *string           `json:"rule_key"`
	Scope            map[string]string `json:"scope"`
	Severity         string            `json:"severity"`
	SignalMapping    signalMappingJSON `json:"signal_mapping"`
}

type alertResponse struct {
	alertPayload
	Id            int64              `json:"id"`
	SignalMapping *signalMappingJSON `json:"signal_mapping"`
	Token         string             `json:"token"`
}

type signalMappingJSON struct {
	CategoryColumn    *string            `json:"category_column,omitempty"`
	DescriptionColumn *string            `json:"description_column,omitempty"`
	Identifiers       map[string]string  `json:"identifiers,omitempty"`
	MetadataColumns   []string           `json:"metadata_columns,omitempty"`
	Notebook          []notebookCellJSON `json:"notebook,omitempty"`
	Preview           *previewJSON       `json:"preview,omitempty"`
	SeverityColumn    *string            `json:"severity_column,omitempty"`
	TitleColumn       *string            `json:"title_column,omitempty"`
	WindowSeconds     *int64             `json:"window_seconds,omitempty"`
}

type previewJSON struct {
	Lql *string `json:"lql,omitempty"`
}

type notebookCellJSON struct {
	Source string `json:"source"`
	Sql    string `json:"sql"`
	Title  string `json:"title"`
}

func alertModelToPayload(ctx context.Context, data *AlertResourceModel) ([]byte, diag.Diagnostics) {
	var diags diag.Diagnostics

	payload := alertPayload{
		Category:         data.Category.ValueString(),
		Cron:             data.Cron.ValueString(),
		Description:      data.Description.ValueStringPointer(),
		Enabled:          data.Enabled.ValueBool(),
		GroupBy:          []string{},
		Language:         data.Language.ValueString(),
		Managed:          data.Managed.ValueBool(),
		Name:             data.Name.ValueString(),
		Query:            data.Query.ValueString(),
		ResolveAfterRuns: data.ResolveAfterRuns.ValueInt64(),
		RuleKey:          data.RuleKey.ValueStringPointer(),
		Scope:            map[string]string{},
		Severity:         data.Severity.ValueString(),
	}

	diags.Append(data.GroupBy.ElementsAs(ctx, &payload.GroupBy, false)...)
	diags.Append(data.Scope.ElementsAs(ctx, &payload.Scope, false)...)

	if sm := data.SignalMapping; sm != nil {
		payload.SignalMapping = signalMappingJSON{
			CategoryColumn:    sm.CategoryColumn.ValueStringPointer(),
			DescriptionColumn: sm.DescriptionColumn.ValueStringPointer(),
			SeverityColumn:    sm.SeverityColumn.ValueStringPointer(),
			TitleColumn:       sm.TitleColumn.ValueStringPointer(),
			WindowSeconds:     sm.WindowSeconds.ValueInt64Pointer(),
		}
		diags.Append(sm.Identifiers.ElementsAs(ctx, &payload.SignalMapping.Identifiers, false)...)
		diags.Append(sm.MetadataColumns.ElementsAs(ctx, &payload.SignalMapping.MetadataColumns, false)...)

		if sm.Preview != nil {
			payload.SignalMapping.Preview = &previewJSON{Lql: sm.Preview.Lql.ValueStringPointer()}
		}

		for _, cell := range sm.Notebook {
			payload.SignalMapping.Notebook = append(payload.SignalMapping.Notebook, notebookCellJSON{
				Source: cell.Source.ValueString(),
				Sql:    cell.Sql.ValueString(),
				Title:  cell.Title.ValueString(),
			})
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		diags.AddError("Encoding Error", err.Error())
	}

	return body, diags
}

type alertHTTPResponse interface {
	StatusCode() int
}

func alertResponseToModel(
	ctx context.Context, op string, httpResp alertHTTPResponse, err error, wantStatus int, data *AlertResourceModel,
) diag.Diagnostics {
	var diags diag.Diagnostics

	if err != nil {
		diags.AddError("Client Error", fmt.Sprintf("Unable to %s alert, got error: %s", op, err))
		return diags
	}

	raw := alertResponseBody(httpResp)
	if httpResp.StatusCode() != wantStatus {
		diags.AddError("Client Error", fmt.Sprintf("Unable to %s alert, got status %d: %s", op, httpResp.StatusCode(), raw))
		return diags
	}

	var result alertResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		diags.AddError("Decoding Error", fmt.Sprintf("Unable to decode alert: %s", err))
		return diags
	}

	data.Id = types.Int64Value(result.Id)
	data.Token = types.StringValue(result.Token)
	data.Name = types.StringValue(result.Name)
	data.Description = types.StringPointerValue(result.Description)
	data.Language = types.StringValue(result.Language)
	data.Query = types.StringValue(result.Query)
	data.Cron = types.StringValue(result.Cron)
	data.RuleKey = types.StringPointerValue(result.RuleKey)
	data.Managed = types.BoolValue(result.Managed)
	data.Severity = types.StringValue(result.Severity)
	data.Category = types.StringValue(result.Category)
	data.ResolveAfterRuns = types.Int64Value(result.ResolveAfterRuns)
	data.Enabled = types.BoolValue(result.Enabled)

	var d diag.Diagnostics
	data.GroupBy, d = types.ListValueFrom(ctx, types.StringType, nonNilStrings(result.GroupBy))
	diags.Append(d...)
	data.Scope, d = types.MapValueFrom(ctx, types.StringType, nonNilMap(result.Scope))
	diags.Append(d...)

	data.SignalMapping, d = signalMappingToModel(ctx, result.SignalMapping)
	diags.Append(d...)

	return diags
}

func alertResponseBody(httpResp alertHTTPResponse) []byte {
	switch r := httpResp.(type) {
	case *api.LogflareWebApiAlertControllerCreateResponse:
		return r.Body
	case *api.LogflareWebApiAlertControllerShowResponse:
		return r.Body
	case *api.LogflareWebApiAlertControllerUpdateResponse:
		return r.Body
	default:
		return nil
	}
}

func signalMappingToModel(ctx context.Context, sm *signalMappingJSON) (*AlertSignalMappingModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	if sm == nil || isEmptySignalMapping(sm) {
		return nil, diags
	}

	model := &AlertSignalMappingModel{
		CategoryColumn:    types.StringPointerValue(sm.CategoryColumn),
		DescriptionColumn: types.StringPointerValue(sm.DescriptionColumn),
		Identifiers:       types.MapNull(types.StringType),
		MetadataColumns:   types.ListNull(types.StringType),
		SeverityColumn:    types.StringPointerValue(sm.SeverityColumn),
		TitleColumn:       types.StringPointerValue(sm.TitleColumn),
		WindowSeconds:     types.Int64PointerValue(sm.WindowSeconds),
	}

	var d diag.Diagnostics
	if len(sm.Identifiers) > 0 {
		model.Identifiers, d = types.MapValueFrom(ctx, types.StringType, sm.Identifiers)
		diags.Append(d...)
	}

	if len(sm.MetadataColumns) > 0 {
		model.MetadataColumns, d = types.ListValueFrom(ctx, types.StringType, sm.MetadataColumns)
		diags.Append(d...)
	}

	if sm.Preview != nil {
		model.Preview = &AlertPreviewModel{Lql: types.StringPointerValue(sm.Preview.Lql)}
	}

	for _, cell := range sm.Notebook {
		model.Notebook = append(model.Notebook, AlertNotebookCellModel{
			Source: types.StringValue(cell.Source),
			Sql:    types.StringValue(cell.Sql),
			Title:  types.StringValue(cell.Title),
		})
	}

	return model, diags
}

func isEmptySignalMapping(sm *signalMappingJSON) bool {
	return sm.CategoryColumn == nil && sm.DescriptionColumn == nil && len(sm.Identifiers) == 0 &&
		len(sm.MetadataColumns) == 0 && len(sm.Notebook) == 0 && sm.Preview == nil &&
		sm.SeverityColumn == nil && sm.TitleColumn == nil && sm.WindowSeconds == nil
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func nonNilMap(values map[string]string) map[string]string {
	if values == nil {
		return map[string]string{}
	}
	return values
}
