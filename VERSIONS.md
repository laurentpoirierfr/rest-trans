# Changelog

## v0.2.0

**Date :** 27 juillet 2026

### Nouvelles fonctionnalités

- **Bulk upsert** — Upsert batch avec `ON CONFLICT` sur n'importe quelles colonnes via `?on_conflict=col1,col2` (pas juste la PK)
- **Metrics/Prometheus** — Endpoint `GET /ops/metrics` avec compteurs de requêtes, histogrammes de latence, compteurs d'erreurs et gauge de requêtes en cours
- **Health check** — `GET /ops/liveness` (probe simple) et `GET /ops/readiness` (ping DB)
- **Logging structuré** — Migration de `log.Printf` vers `log/slog` avec sortie JSON structurée
- **Graceful shutdown** — Arrêt propre du serveur HTTP, du transaction manager et des connexions DB sur SIGINT/SIGTERM
- **Hot reload** — Watcher le schéma PostgreSQL pour détecter les changements (nouvelles tables, colonnes, fonctions) sans redémarrage
- **SSE (Server-Sent Events)** — `GET /:schema/:table/_stream` pour du real-time via PostgreSQL LISTEN/NOTIFY
  - Package `internal/notification` avec `Hub` (fan-out SSE) et `Listener` (écoute PG)
  - Trigger SQL `rest_notify()` à attacher aux tables pour les notifications INSERT/UPDATE/DELETE
  - Channel au format `rest_<schema>_<table>` avec payload JSON complet
  - Lifecycle intégré au graceful shutdown

### Améliorations

- `SchemaStore` : ajout de `sync.RWMutex` pour l'accès concurrent-safe
- `http.Server` avec `ReadTimeout`, `WriteTimeout`, `IdleTimeout`

### Métriques exposées

| Métrique | Type | Labels |
|----------|------|--------|
| `resttrans_http_requests_total` | Counter | method, path, status |
| `resttrans_http_request_duration_seconds` | Histogram | method, path |
| `resttrans_http_errors_total` | Counter | method, path, status |
| `resttrans_http_inflight_requests` | Gauge | method |

### Configuration ajoutée

```yaml
hot_reload:
  enabled: false
  interval: 30s
```

---

## v0.1.0

**Date :** 26 juillet 2026

### Fonctionnalités initiales

- **Introspection automatique** — Tables, vues, clés primaires, clés étrangères, contraintes uniques
- **CRUD complet** — GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS sur `/:schema/:table`
- **Filtrage** — `eq`, `neq`, `gt`, `gte`, `lt`, `lte`, `like`, `ilike`, `in`, `is`, opérateurs JSONB
- **Opérateurs logiques** — `_and`, `_or`, `_not.and`, `_not.or`
- **Sélection de colonnes** — `_select=col1,col2`
- **Tri** — `_order=col.desc,col2.asc`
- **Pagination** — `_limit`, `_offset` avec headers `Content-Range` et `Range`
- **Réponses JSON/CSV** via header `Accept`
- **Resource Embedding** — `_select=*,tasks(title,status)` avec LEFT JOINs automatiques
- **RPC / Fonctions stockées** — `POST /:schema/rpc/:function`
- **Upsert** — `Prefer: resolution=merge-duplicates` (sur PK uniquement)
- **PUT singleton** — Upsert sur clé primaire
- **Agrégats** — `_select=count(*),avg(age)`
- **Documentation OpenAPI** — `GET /openapi.json` + Swagger UI
- **Permissions HTTP** — Par table via commentaires PostgreSQL (`@allow`, `@deny`) ou `config.yaml`
- **Config Viper** — Fichier YAML + overrides par variables d'environnement
- **Pool de connexions** — `max_open`, `max_idle`, `conn_max_life`, `conn_max_idle`
- **Tables cachées** — `hidden_tables` avec wildcards pour masquer de l'OpenAPI et `/info`
- **Transactions distribuées** — Pattern Saga avec staging, commit atomique et rollback
- **CORS** configurable
- **Erreurs structurées** au format PostgREST
