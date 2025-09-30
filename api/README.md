# oapi-codegen api config

We need to generate the yaml file for logflare, to do so run in logflare app:

```bash
mix openapi.spec.yaml --spec LogflareWeb.ApiSpec
```

That will generate `openapi.yaml` place it on this folder


# Notes:

Due to open api spex limitations the `/api/endpoints/query/name/{name}` endpoint needs a manual patching to replace `name: token_or_name` to `name: name`
