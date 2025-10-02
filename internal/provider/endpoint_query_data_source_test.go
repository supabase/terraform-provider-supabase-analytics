// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccEndpointsDataSource(t *testing.T) {
	currentTime := time.Now()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: providerConfig + testAccEndpointsDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Example result: {"result":[{"date":["2025-10-02"]}]}
					resource.TestCheckResourceAttr("data.logflare_endpoint_query.test", "result.#", "1"),
					resource.TestCheckResourceAttr("data.logflare_endpoint_query.test", "result.0.date.0", currentTime.Format(time.DateOnly)),
				),
			},
		},
	})
}

const testAccEndpointsDataSourceConfig = `
resource "logflare_endpoint" "endpoint_test" {
	name = "endpoint_test"
	query = "select current_date as date"
}

data "logflare_endpoint_query" "test" {
	name_or_token = logflare_endpoint.endpoint_test.name
}
`
