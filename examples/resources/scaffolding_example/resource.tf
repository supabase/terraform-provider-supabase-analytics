resource "logflare_endpoint" "example" {
  name  = "my_cool_endpoint"
  query = "select current_date as date"
}
