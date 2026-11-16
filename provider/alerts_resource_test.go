// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAlertResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + testAlertResourceConfig("major", "Errors by function", "default"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("logflare_alert.rule", "name", "Edge Function errors"),
					resource.TestCheckResourceAttr("logflare_alert.rule", "language", "pg_sql"),
					resource.TestCheckResourceAttr("logflare_alert.rule", "rule_key", "tf-test.edge-function-5xx"),
					resource.TestCheckResourceAttr("logflare_alert.rule", "managed", "true"),
					resource.TestCheckResourceAttr("logflare_alert.rule", "severity", "major"),
					resource.TestCheckResourceAttr("logflare_alert.rule", "category", "performance"),
					resource.TestCheckResourceAttr("logflare_alert.rule", "group_by.0", "function_id"),
					resource.TestCheckResourceAttr("logflare_alert.rule", "scope.project", "default"),
					resource.TestCheckResourceAttr("logflare_alert.rule", "resolve_after_runs", "3"),
					resource.TestCheckResourceAttr("logflare_alert.rule", "signal_mapping.identifiers.function", "function_id"),
					resource.TestCheckResourceAttr("logflare_alert.rule", "signal_mapping.window_seconds", "300"),
					resource.TestCheckResourceAttr("logflare_alert.rule", "signal_mapping.preview.lql", "f:function_edge_logs m.function_id:{{function_id}}"),
					resource.TestCheckResourceAttr("logflare_alert.rule", "signal_mapping.notebook.0.title", "Errors by function"),
					resource.TestCheckResourceAttr("logflare_alert.rule", "signal_mapping.notebook.0.source", "database"),
					resource.TestCheckResourceAttrSet("logflare_alert.rule", "id"),
					resource.TestCheckResourceAttrSet("logflare_alert.rule", "token"),
					resource.TestCheckResourceAttr("logflare_alert.minimal", "severity", "warning"),
					resource.TestCheckResourceAttr("logflare_alert.minimal", "category", "health"),
					resource.TestCheckResourceAttr("logflare_alert.minimal", "managed", "false"),
					resource.TestCheckResourceAttr("logflare_alert.minimal", "enabled", "true"),
					resource.TestCheckResourceAttr("logflare_alert.minimal", "resolve_after_runs", "2"),
					resource.TestCheckNoResourceAttr("logflare_alert.minimal", "signal_mapping"),
					resource.TestCheckNoResourceAttr("logflare_alert.minimal", "rule_key"),
				),
			},
			{
				Config: providerConfig + testAlertResourceConfig("critical", "Recent errors", "staging"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("logflare_alert.rule", "severity", "critical"),
					resource.TestCheckResourceAttr("logflare_alert.rule", "scope.project", "staging"),
					resource.TestCheckResourceAttr("logflare_alert.rule", "signal_mapping.notebook.0.title", "Recent errors"),
				),
			},
			{
				Config: providerConfig + testAlertResourceWithoutMappingConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("logflare_alert.rule", "signal_mapping"),
					resource.TestCheckNoResourceAttr("logflare_alert.rule", "description"),
					resource.TestCheckResourceAttr("logflare_alert.rule", "group_by.#", "0"),
					resource.TestCheckResourceAttr("logflare_alert.rule", "enabled", "false"),
				),
			},
			{
				Config: providerConfig + testAlertResourceConfig("critical", "Recent errors", "staging"),
			},
			{
				ResourceName:      "logflare_alert.rule",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      "logflare_alert.minimal",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAlertResourceConfig(severity, notebookTitle, project string) string {
	return fmt.Sprintf(`
resource "logflare_alert" "rule" {
  name               = "Edge Function errors"
  description        = "An Edge Function returned 5xx responses."
  language           = "pg_sql"
  query              = "select 'hello' as function_id"
  cron               = "*/5 * * * *"
  rule_key           = "tf-test.edge-function-5xx"
  managed            = true
  severity           = %[1]q
  category           = "performance"
  group_by           = ["function_id"]
  scope              = { project = %[3]q }
  resolve_after_runs = 3

  signal_mapping = {
    identifiers      = { function = "function_id" }
    metadata_columns = ["function_id"]
    window_seconds   = 300
    preview          = { lql = "f:function_edge_logs m.function_id:{{function_id}}" }
    notebook = [
      { title = %[2]q, source = "database", sql = "select {{literal:function}} as function_id" },
    ]
  }
}

resource "logflare_alert" "minimal" {
  name     = "Minimal alert"
  language = "pg_sql"
  query    = "select 1 as one"
  cron     = "*/5 * * * *"
}
`, severity, notebookTitle, project)
}

const testAlertResourceWithoutMappingConfig = `
resource "logflare_alert" "rule" {
  name     = "Edge Function errors"
  language = "pg_sql"
  query    = "select 'hello' as function_id"
  cron     = "*/5 * * * *"
  rule_key = "tf-test.edge-function-5xx"
  enabled  = false
}

resource "logflare_alert" "minimal" {
  name     = "Minimal alert"
  language = "pg_sql"
  query    = "select 1 as one"
  cron     = "*/5 * * * *"
}
`
