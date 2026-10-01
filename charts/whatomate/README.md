# Whatomate Helm chart

Deploys [Whatomate](https://github.com/shridarpatil/whatomate) on Kubernetes. PostgreSQL and Redis
are not bundled: point the chart at existing instances (managed services, or charts such as
`bitnami/postgresql` and `bitnami/redis`).

## Install

```bash
kubectl create namespace whatomate
kubectl -n whatomate create secret generic whatomate \
  --from-literal=db-password='...' \
  --from-literal=jwt-secret="$(openssl rand -hex 32)" \
  --from-literal=encryption-key="$(openssl rand -hex 32)"

helm install whatomate ./charts/whatomate -n whatomate -f my-values.yaml
```

`my-values.yaml`:

```yaml
config:
  database:
    host: postgres.whatomate.svc
  redis:
    host: redis-master.whatomate.svc
secrets:
  existingSecret: whatomate
  keys:
    database.password: db-password
    jwt.secret: jwt-secret
    app.encryption_key: encryption-key
ingress:
  enabled: true
  className: nginx
  hosts:
    - host: whatomate.example.com
```

## Configuration

`config` is a nested map using the same sections and keys as `config.example.toml`. Each entry is
rendered as a `WHATOMATE_<SECTION>__<KEY>` environment variable (for example `rate_limit.enabled`
becomes `WHATOMATE_RATE_LIMIT__ENABLED`), so key names always match what the app expects. Empty
values are skipped so they never override `configToml`.

`config.server.allowed_origins` is required when `config.app.environment` is `production`. If it is
empty, the chart derives it from `ingress.hosts`, and `helm install` fails with a clear message when it
cannot.

Secret settings use the same `section.key` paths:

| Value | Purpose |
| --- | --- |
| `secrets.existingSecret` + `secrets.keys` | Map config paths to keys in a Secret you manage |
| `secrets.values` | Let the chart create the Secret (quick starts only) |
| `extraEnv`, `extraEnvFrom`, `extraVolumes`, `extraVolumeMounts` | Wire in external secret stores, CSI drivers or anything else |

Other values:

| Value | Default | Notes |
| --- | --- | --- |
| `image.tag` | chart `appVersion` | Pin a version; avoid `latest` |
| `migrate` | `true` | Runs migrations on start. Back up the database before upgrading |
| `persistence.size` | `10Gi` | Uploaded media. Watch usage, the app fails to store media when it is full |
| `persistence.keep` | `true` | Keeps the volume on `helm uninstall` |
| `strategy` | `{}` (RollingUpdate) | Use `type: Recreate` if the pod can move to another node with a ReadWriteOnce volume |

The deployment runs a single replica because uploads live on a ReadWriteOnce volume.
