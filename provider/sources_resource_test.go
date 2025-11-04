// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccSourcesResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + testAccSourcesResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("logflare_source.source_test", "name", "my-cool-source"),
					resource.TestCheckResourceAttr("logflare_source.source_test", "favorite", "true"),
					resource.TestCheckResourceAttrSet("logflare_source.source_test", "id"),
					resource.TestCheckResourceAttrSet("logflare_source.source_test", "token"),
				),
			},
		},
	})
}

const testAccSourcesResourceConfig = `
resource "logflare_source" "source_test" {
	name = "my-cool-source"
    favorite = true
}
`
