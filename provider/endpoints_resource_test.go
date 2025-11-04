// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestEndpointsResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: providerConfig + testEndpointsresourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("logflare_endpoint.endpoint_test", "name", "my_cool_endpoint"),
					resource.TestCheckResourceAttr("logflare_endpoint.endpoint_test", "enable_auth", "true"),
				),
			},
		},
	})
}

const testEndpointsresourceConfig = `
resource "logflare_endpoint" "endpoint_test" {
	name = "my_cool_endpoint"
	query = "select current_date as date"
}
`
