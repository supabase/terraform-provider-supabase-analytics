# Page the ops webhook when a security advisor alert issue opens in the default project,
# and again every hour while it stays open and unacknowledged.
resource "logflare_alert_notification_policy" "advisor_pages" {
  name                    = "advisor-pages"
  rule_keys               = ["supabase.advisor.*"]
  min_severity            = "major"
  categories              = ["security"]
  identifiers             = { project = "default" }
  events                  = ["opened", "reopened", "repeat", "resolved"]
  repeat_interval_seconds = 3600
  receivers               = ["ops-webhook"]
}
