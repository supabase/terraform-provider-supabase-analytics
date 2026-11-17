# Alert issue rules for the self-hosted Logflare. Changes go through Terraform only;
# the Logflare UI shows these rules read-only.
terraform {
  required_providers {
    logflare = {
      source = "logflare/logflare"
    }
  }
}

provider "logflare" {
  host         = var.logflare_host
  access_token = var.logflare_access_token
}

variable "logflare_host" {
  type    = string
  default = "http://localhost:4000"
}

variable "logflare_access_token" {
  type      = string
  sensitive = true
}

variable "project" {
  type    = string
  default = "default"
}

resource "logflare_source" "this" {
  for_each = toset(["function_edge_logs", "function_logs", "advisor_lints"])
  name     = each.key
}

resource "logflare_alert" "edge_function_5xx" {
  name        = "Edge Function errors"
  description = "An Edge Function returned 5xx responses in the last hour."
  language    = "pg_sql"
  query       = "select body -> 'metadata' ->> 'function_id' as function_id, count(*) as errors from function_edge_logs where timestamp > now() - '1 hour'::interval and (body -> 'metadata' ->> 'status_code')::int >= 500 group by 1"
  cron        = "*/5 * * * *"
  rule_key    = "supabase.edge-function-5xx"
  managed     = true
  severity    = "major"
  category    = "health"
  group_by    = ["function_id"]
  scope       = { project = var.project }

  signal_mapping = {
    preview = { lql = "f:function_edge_logs m.function_id:{{function_id}} m.status_code:>=500" }
  }

  depends_on = [logflare_source.this]
}

resource "logflare_alert" "edge_function_latency" {
  name        = "Slow Edge Function responses"
  description = "Edge Function requests took longer than 5 seconds."
  language    = "pg_sql"
  query       = "select body -> 'metadata' ->> 'function_id' as function_id, count(*) as slow from function_edge_logs where timestamp > now() - '1 hour'::interval and (body -> 'metadata' ->> 'execution_time_ms')::int > 5000 group by 1 having count(*) > 20"
  cron        = "*/5 * * * *"
  rule_key    = "supabase.edge-function-latency"
  managed     = true
  severity    = "warning"
  category    = "performance"
  group_by    = ["function_id"]
  scope       = { project = var.project }

  signal_mapping = {
    preview = { lql = "f:function_edge_logs m.function_id:{{function_id}} m.execution_time_ms:>5000" }
  }

  depends_on = [logflare_source.this]
}

# One managed rule per Supabase advisor lint. Advisors ingest their findings into advisor_lints.
variable "advisor_lints" {
  type = map(object({
    name        = string
    description = string
    query       = string
    severity    = string
    category    = string
    notebook = list(object({
      title  = string
      source = string
      sql    = string
    }))
  }))

  default = {
    rls_disabled_in_public = {
      name        = "RLS Disabled in Public"
      description = "Detects cases where row level security (RLS) has not been enabled on tables in schemas exposed to PostgREST"
      query       = "select body -> 'metadata' ->> 'schema' as schema, coalesce(body -> 'metadata' ->> 'name', body -> 'metadata' ->> 'entity') as entity, max(body ->> 'title') || coalesce(': ' || (body -> 'metadata' ->> 'schema') || '.' || (body -> 'metadata' ->> 'name'), '') as title, max(body ->> 'detail') as detail, max(body ->> 'remediation') as remediation from advisor_lints where timestamp > now() - '1 day'::interval and body ->> 'name' = 'rls_disabled_in_public' group by 1, 2, body -> 'metadata'"
      severity    = "critical"
      category    = "security"
      notebook = [
        {
          title  = "Rows readable with the anon key"
          source = "database"
          sql    = "select count(*) as exposed_rows from {{ident:schema}}.{{ident:table}};"
        },
        {
          title  = "Enable RLS"
          source = "database"
          sql    = "alter table {{ident:schema}}.{{ident:table}} enable row level security;"
        },
      ]
    }
    policy_exists_rls_disabled = {
      name        = "Policy Exists RLS Disabled"
      description = "Detects cases where row level security (RLS) policies have been created, but RLS has not been enabled for the underlying table."
      query       = "select body -> 'metadata' ->> 'schema' as schema, coalesce(body -> 'metadata' ->> 'name', body -> 'metadata' ->> 'entity') as entity, max(body ->> 'title') || coalesce(': ' || (body -> 'metadata' ->> 'schema') || '.' || (body -> 'metadata' ->> 'name'), '') as title, max(body ->> 'detail') as detail, max(body ->> 'remediation') as remediation from advisor_lints where timestamp > now() - '1 day'::interval and body ->> 'name' = 'policy_exists_rls_disabled' group by 1, 2, body -> 'metadata'"
      severity    = "critical"
      category    = "security"
      notebook = [
        {
          title  = "Policies that are not enforced"
          source = "database"
          sql    = "select policyname, permissive, roles, cmd, qual from pg_policies where schemaname = {{literal:schema}} and tablename = {{literal:table}};"
        },
        {
          title  = "Enable RLS"
          source = "database"
          sql    = "alter table {{ident:schema}}.{{ident:table}} enable row level security;"
        },
      ]
    }
    security_definer_view = {
      name        = "Security Definer View"
      description = "Detects views defined with the SECURITY DEFINER property. These views enforce Postgres permissions and row level security policies (RLS) of the view creator, rather than that of the querying user"
      query       = "select body -> 'metadata' ->> 'schema' as schema, coalesce(body -> 'metadata' ->> 'name', body -> 'metadata' ->> 'entity') as entity, max(body ->> 'title') || coalesce(': ' || (body -> 'metadata' ->> 'schema') || '.' || (body -> 'metadata' ->> 'name'), '') as title, max(body ->> 'detail') as detail, max(body ->> 'remediation') as remediation from advisor_lints where timestamp > now() - '1 day'::interval and body ->> 'name' = 'security_definer_view' group by 1, 2, body -> 'metadata'"
      severity    = "critical"
      category    = "security"
      notebook = [
        {
          title  = "View definition"
          source = "database"
          sql    = "select pg_get_viewdef(format('%I.%I', {{literal:schema}}, {{literal:table}})::regclass, true) as definition;"
        },
        {
          title  = "Run the view as the querying user"
          source = "database"
          sql    = "alter view {{ident:schema}}.{{ident:table}} set (security_invoker = on);"
        },
      ]
    }
    function_search_path_mutable = {
      name        = "Function Search Path Mutable"
      description = "Detects functions where the search_path parameter is not set."
      query       = "select body -> 'metadata' ->> 'schema' as schema, coalesce(body -> 'metadata' ->> 'name', body -> 'metadata' ->> 'entity') as entity, max(body ->> 'title') || coalesce(': ' || (body -> 'metadata' ->> 'schema') || '.' || (body -> 'metadata' ->> 'name'), '') as title, max(body ->> 'detail') as detail, max(body ->> 'remediation') as remediation from advisor_lints where timestamp > now() - '1 day'::interval and body ->> 'name' = 'function_search_path_mutable' group by 1, 2, body -> 'metadata'"
      severity    = "warning"
      category    = "security"
      notebook = [
        {
          title  = "Function settings"
          source = "database"
          sql    = "select p.proname, p.prosecdef as security_definer, p.proconfig from pg_proc p join pg_namespace n on n.oid = p.pronamespace where n.nspname = {{literal:schema}} and p.proname = {{literal:table}};"
        },
        {
          title  = "Pin the search_path"
          source = "database"
          sql    = "alter function {{ident:schema}}.{{ident:table}}() set search_path = '';"
        },
      ]
    }
    auth_leaked_password_protection = {
      name        = "Leaked Password Protection Disabled"
      description = "Leaked password protection is currently disabled."
      query       = "select body -> 'metadata' ->> 'schema' as schema, coalesce(body -> 'metadata' ->> 'name', body -> 'metadata' ->> 'entity') as entity, max(body ->> 'title') || coalesce(': ' || (body -> 'metadata' ->> 'schema') || '.' || (body -> 'metadata' ->> 'name'), '') as title, max(body ->> 'detail') as detail, max(body ->> 'remediation') as remediation from advisor_lints where timestamp > now() - '1 day'::interval and body ->> 'name' = 'auth_leaked_password_protection' group by 1, 2, body -> 'metadata'"
      severity    = "warning"
      category    = "security"
      notebook    = []
    }
    auth_rls_initplan = {
      name        = "Auth RLS Initialization Plan"
      description = "Detects if calls to `current_setting()` and `auth.<function>()` in RLS policies are being unnecessarily re-evaluated for each row"
      query       = "select body -> 'metadata' ->> 'schema' as schema, coalesce(body -> 'metadata' ->> 'name', body -> 'metadata' ->> 'entity') as entity, max(body ->> 'title') || coalesce(': ' || (body -> 'metadata' ->> 'schema') || '.' || (body -> 'metadata' ->> 'name'), '') as title, max(body ->> 'detail') as detail, max(body ->> 'remediation') as remediation from advisor_lints where timestamp > now() - '1 day'::interval and body ->> 'name' = 'auth_rls_initplan' group by 1, 2, body -> 'metadata'"
      severity    = "warning"
      category    = "performance"
      notebook = [
        {
          title  = "Policies on this table"
          source = "database"
          sql    = "select policyname, permissive, roles, cmd, qual from pg_policies where schemaname = {{literal:schema}} and tablename = {{literal:table}};"
        },
      ]
    }
    multiple_permissive_policies = {
      name        = "Multiple Permissive Policies"
      description = "Detects if multiple permissive row level security policies are present on a table for the same `role` and `action` (e.g. insert)."
      query       = "select body -> 'metadata' ->> 'schema' as schema, coalesce(body -> 'metadata' ->> 'name', body -> 'metadata' ->> 'entity') as entity, max(body ->> 'title') || coalesce(': ' || (body -> 'metadata' ->> 'schema') || '.' || (body -> 'metadata' ->> 'name'), '') as title, max(body ->> 'detail') as detail, max(body ->> 'remediation') as remediation from advisor_lints where timestamp > now() - '1 day'::interval and body ->> 'name' = 'multiple_permissive_policies' group by 1, 2, body -> 'metadata'"
      severity    = "warning"
      category    = "performance"
      notebook = [
        {
          title  = "Overlapping permissive policies"
          source = "database"
          sql    = "select policyname, permissive, roles, cmd, qual from pg_policies where schemaname = {{literal:schema}} and tablename = {{literal:table}};"
        },
      ]
    }
    unindexed_foreign_keys = {
      name        = "Unindexed foreign keys"
      description = "Identifies foreign key constraints without a covering index, which can impact database performance."
      query       = "select body -> 'metadata' ->> 'schema' as schema, coalesce(body -> 'metadata' ->> 'name', body -> 'metadata' ->> 'entity') as entity, max(body ->> 'title') || coalesce(': ' || (body -> 'metadata' ->> 'schema') || '.' || (body -> 'metadata' ->> 'name'), '') as title, max(body ->> 'detail') as detail, max(body ->> 'remediation') as remediation from advisor_lints where timestamp > now() - '1 day'::interval and body ->> 'name' = 'unindexed_foreign_keys' group by 1, 2, body -> 'metadata'"
      severity    = "info"
      category    = "performance"
      notebook = [
        {
          title  = "Foreign key definition"
          source = "database"
          sql    = "select conname, pg_get_constraintdef(c.oid) as definition from pg_constraint c where c.conrelid = format('%I.%I', {{literal:schema}}, {{literal:table}})::regclass and c.contype = 'f';"
        },
        {
          title  = "Sequential vs index scans"
          source = "database"
          sql    = "select relname, seq_scan, idx_scan, n_live_tup from pg_stat_user_tables where schemaname = {{literal:schema}} and relname = {{literal:table}};"
        },
      ]
    }
  }
}

resource "logflare_alert" "advisor" {
  for_each = var.advisor_lints

  name        = each.value.name
  description = each.value.description
  language    = "pg_sql"
  query       = each.value.query
  cron        = "*/5 * * * *"
  rule_key    = "supabase.advisor.${each.key}"
  managed     = true
  severity    = each.value.severity
  category    = each.value.category
  group_by    = ["entity"]
  scope       = { project = var.project }

  signal_mapping = {
    identifiers        = { schema = "schema", table = "entity" }
    title_column       = "title"
    description_column = "detail"
    metadata_columns   = ["remediation", "detail"]
    preview            = { lql = "f:advisor_lints name:${each.key}" }
    notebook           = length(each.value.notebook) > 0 ? each.value.notebook : null
  }

  depends_on = [logflare_source.this]
}

# Notify the ops webhook when an Edge Function alert issue opens, reopens or resolves.
resource "logflare_alert_notification_policy" "edge_function_pages" {
  name         = "edge-function-pages"
  rule_keys    = ["supabase.edge-function-*", "custom.*"]
  min_severity = "warning"
  identifiers  = { project = var.project }
  events       = ["opened", "reopened", "resolved"]
  receivers    = ["ops-webhook"]
}
