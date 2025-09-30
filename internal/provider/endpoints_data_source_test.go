// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccEndpointsDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: providerConfig + testAccEndpointsDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.logflare_endpoints.test", "results.#", "1"),
					resource.TestCheckResourceAttr("data.logflare_endpoints.test", "results.0.timestamp", ""),
					resource.TestCheckResourceAttr("data.logflare_endpoints.test", "results.0.event_message", "{}"),
				),
			},
		},
	})
}

const testAccEndpointsDataSourceConfig = `
data "logflare_endpoints" "test" {
	name_or_token = "cbb957ed-913e-4b21-bdc4-150d74d26e57"
}
`
