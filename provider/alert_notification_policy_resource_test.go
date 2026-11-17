// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestNotificationPolicyPayloadRoundTrip(t *testing.T) {
	ctx := context.Background()
	strings := func(values ...string) types.List {
		elems := make([]attr.Value, len(values))
		for i, v := range values {
			elems[i] = types.StringValue(v)
		}
		return types.ListValueMust(types.StringType, elems)
	}

	model := NotificationPolicyResourceModel{
		Name:                  types.StringValue("advisor-pages"),
		Enabled:               types.BoolValue(true),
		RuleKeys:              strings("supabase.advisor.*"),
		MinSeverity:           types.StringNull(),
		Categories:            strings(),
		Identifiers:           types.MapValueMust(types.StringType, map[string]attr.Value{"project": types.StringValue("default")}),
		Events:                strings("opened", "repeat"),
		RepeatIntervalSeconds: types.Int64Value(3600),
		Receivers:             strings("ops-webhook"),
	}

	body, diags := notificationPolicyModelToPayload(ctx, &model)
	if diags.HasError() {
		t.Fatalf("payload: %v", diags)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}

	if payload["min_severity"] != nil {
		t.Errorf("min_severity should be sent as null to clear it, got %v", payload["min_severity"])
	}
	if payload["repeat_interval_seconds"] != float64(3600) {
		t.Errorf("repeat_interval_seconds = %v", payload["repeat_interval_seconds"])
	}
	if cats, ok := payload["categories"].([]any); !ok || len(cats) != 0 {
		t.Errorf("categories should be an empty list, got %v", payload["categories"])
	}

	response := []byte(`{"id": 7, "name": "advisor-pages", "enabled": true, "rule_keys": ["supabase.advisor.*"],
		"min_severity": null, "categories": [], "identifiers": {"project": "default"},
		"events": ["opened", "repeat"], "repeat_interval_seconds": 3600, "receivers": ["ops-webhook"]}`)

	var read NotificationPolicyResourceModel
	if diags := notificationPolicyJSONToModel(ctx, response, &read); diags.HasError() {
		t.Fatalf("model: %v", diags)
	}

	if read.ID.ValueInt64() != 7 || !read.MinSeverity.IsNull() || read.RepeatIntervalSeconds.ValueInt64() != 3600 {
		t.Errorf("unexpected model: %+v", read)
	}
	if !read.Receivers.Equal(model.Receivers) || !read.Identifiers.Equal(model.Identifiers) {
		t.Errorf("lists or maps differ: %+v", read)
	}
}

func TestAlertNotificationPolicyResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + testNotificationPolicyConfig("major", 3600),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("logflare_alert_notification_policy.pages", "name", "tf-test-advisor-pages"),
					resource.TestCheckResourceAttr("logflare_alert_notification_policy.pages", "rule_keys.0", "supabase.advisor.*"),
					resource.TestCheckResourceAttr("logflare_alert_notification_policy.pages", "min_severity", "major"),
					resource.TestCheckResourceAttr("logflare_alert_notification_policy.pages", "identifiers.project", "default"),
					resource.TestCheckResourceAttr("logflare_alert_notification_policy.pages", "events.#", "2"),
					resource.TestCheckResourceAttr("logflare_alert_notification_policy.pages", "repeat_interval_seconds", "3600"),
					resource.TestCheckResourceAttr("logflare_alert_notification_policy.pages", "receivers.0", "ops-webhook"),
					resource.TestCheckResourceAttrSet("logflare_alert_notification_policy.pages", "id"),
					resource.TestCheckResourceAttr("logflare_alert_notification_policy.minimal", "enabled", "true"),
					resource.TestCheckResourceAttr("logflare_alert_notification_policy.minimal", "events.#", "3"),
					resource.TestCheckResourceAttr("logflare_alert_notification_policy.minimal", "rule_keys.#", "0"),
					resource.TestCheckNoResourceAttr("logflare_alert_notification_policy.minimal", "min_severity"),
					resource.TestCheckNoResourceAttr("logflare_alert_notification_policy.minimal", "repeat_interval_seconds"),
				),
			},
			{
				Config: providerConfig + testNotificationPolicyConfig("critical", 900),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("logflare_alert_notification_policy.pages", "min_severity", "critical"),
					resource.TestCheckResourceAttr("logflare_alert_notification_policy.pages", "repeat_interval_seconds", "900"),
				),
			},
			{
				ResourceName:      "logflare_alert_notification_policy.pages",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testNotificationPolicyConfig(severity string, repeat int) string {
	return fmt.Sprintf(`
resource "logflare_alert_notification_policy" "pages" {
  name                    = "tf-test-advisor-pages"
  rule_keys               = ["supabase.advisor.*"]
  min_severity            = %q
  categories              = ["security"]
  identifiers             = { project = "default" }
  events                  = ["opened", "repeat"]
  repeat_interval_seconds = %d
  receivers               = ["ops-webhook"]
}

resource "logflare_alert_notification_policy" "minimal" {
  name      = "tf-test-minimal"
  receivers = ["ops-webhook"]
}
`, severity, repeat)
}
