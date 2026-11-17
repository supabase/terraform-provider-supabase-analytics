# Alert issues demo

Provisions the self-hosted alert issues demo on a local Logflare with this provider:

- three log sources: `function_edge_logs`, `function_logs` and `advisor_lints`
- ten managed alert issue rules (two Edge Function presets, eight advisor lints), with
  log previews and notebook SQL templates
- one notification policy, sending to the `ops-webhook` receiver

The seed scripts ingest realistic events, and `seed/project.sql` creates the tables the
advisor findings describe, so the SQL cells in Studio return rows.

## Before you start

Run Postgres, Logflare (single tenant, Postgres backend, `LOGFLARE_SUPABASE_MODE=true`) on
:4000, postgres-meta and Studio, as described in supabase/supabase
`apps/studio/components/interfaces/Issues/DEMO.md`. You also need Go, Terraform,
`python3` and `psql`.

## Provision

From the repository root, build the provider where Terraform's dev override can find it:

```sh
go build -o "$HOME/.terraform-dev/terraform-provider-logflare" .
cat > examples/alert-issues-demo/dev.tfrc <<TFRC
provider_installation {
  dev_overrides {
    "logflare/logflare" = "$HOME/.terraform-dev"
  }
  direct {}
}
TFRC
```

Then, from this folder:

```sh
export TF_CLI_CONFIG_FILE=$PWD/dev.tfrc
export TF_VAR_logflare_access_token=my-cool-api-key-123   # LOGFLARE_PRIVATE_ACCESS_TOKEN
terraform apply                                           # no init with dev overrides
seed/seed.sh
psql postgresql://postgres:postgres@localhost:5432/postgres -f seed/project.sql
```

The rules run every 5 minutes, so the alert issues appear in Studio
(`/project/default/issues`) within about 5 minutes of seeding.

## What to try

- **Change a rule:** edit `edge_function_latency` (for example `severity = "major"`) and
  run `terraform apply`. Logflare's `/alerts` page shows the rule read-only, with the new
  values.
- **Custom log rule:** run `seed/seed.sh fnlogs`, then in Studio open Filters → Manage
  alert issue types → New log rule, with source `function_logs`, filter `m.level:error`
  and group by `metadata.function_id`. The rule runs every minute over the last 5
  minutes, so ingest the logs just before creating it.
- **Notifications:** with the notifier and the `ops-webhook` receiver configured (see the
  Studio DEMO.md), 5xx errors from a new function open an alert issue and the webhook
  receives `alert_issue.opened`.

## Reset

```sh
terraform destroy
```

To start completely fresh, also recreate Logflare's database (`mix ecto.reset`) and delete
`terraform.tfstate`.
