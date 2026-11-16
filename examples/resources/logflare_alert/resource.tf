# An alert issue rule: one alert issue per Edge Function returning 5xx responses.
resource "logflare_alert" "edge_function_5xx" {
  name        = "Edge Function errors"
  description = "An Edge Function returned 5xx responses in the last hour."
  language    = "pg_sql"
  query       = <<-SQL
    select body -> 'metadata' ->> 'function_id' as function_id, count(*) as errors
    from function_edge_logs
    where timestamp > now() - interval '1 hour'
      and (body -> 'metadata' ->> 'status_code')::int >= 500
    group by 1
  SQL
  cron        = "*/5 * * * *"
  rule_key    = "supabase.edge-function-5xx"
  managed     = true
  severity    = "major"
  category    = "health"
  group_by    = ["function_id"]
  scope       = { project = "default" }

  signal_mapping = {
    preview = { lql = "f:function_edge_logs m.function_id:{{function_id}} m.status_code:>=500" }
  }
}
