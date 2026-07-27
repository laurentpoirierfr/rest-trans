# Changelog

## v0.5.0

**Date :** 27 juillet 2026

### Nouvelles fonctionnalités

- **View support amélioré** — Les vues sont maintenant correctement gérées en read-only
  - `methodGuard` : les mutations (POST, PUT, PATCH, DELETE) retournent 405 avec code `PGRST201`
  - `corsMiddleware` : exclut les méthodes mutation des `Access-Control-Allow-Methods` pour les vues
  - `HandleOptions` : header `Allow` dynamique (GET, HEAD, OPTIONS pour les vues)
  - OpenAPI spec : omet POST/PUT/PATCH/DELETE pour les vues
  - Ajout d'une vue `active_users` dans le schema de test

### Tests

- `tests/view_test.go` : 9 tests pour valider le comportement des vues
  - `TestViewAllowsGET`, `TestViewAllowsHEAD`
  - `TestViewRejectsPOST`, `TestViewRejectsPUT`, `TestViewRejectsPATCH`, `TestViewRejectsDELETE`
  - `TestViewOptionsHeader`, `TestViewOptionsBody`
  - `TestOpenAPIOmitsMutationsForViews`

---

## v0.4.0

**Date :** 27 juillet 2026

### Nouvelles fonctionnalités

- **Tests avec données isolées** — Chaque test nettoie ses inserts via `t.Cleanup()`
  - Helpers `UniqueSuffix()`, `UniqueEmail()`, `UniqueName()` pour identifiants uniques
  - `SetupTest(t)` enregistre automatiquement le nettoyage (users, projects, project_tasks)
  - Les tests CRUD/RPC/Transactions sont totalement indépendants
  - Les tests de lecture (filter, openapi) utilisent les données seed sans modification

### Améliorations

- **Documentation PROJECT.md** — Document de présentation du projet avec diagrammes mermaid
  - Pitch, architecture, fonctionnalités, comparaison avec PostgREST/Hasura
  - Prêt pour génération de site vitrine par IA
- **Site vitrine docs/index.html** — Page statique de présentation du projet
  - Design dark mode avec gradients et cartes animées
  - Section Architecture avec schéma CSS pur (pas de dépendance externe)
  - 8 cartes de fonctionnalités avec icônes
  - Tableau comparatif rest-trans vs PostgREST vs Hasura
  - Endpoints clés regroupés par catégorie (Documentation, CRUD, SSE, RPC, Transactions, Ops)
  - Badges colorés par méthode HTTP (GET vert, POST orange, DELETE rouge)
  - Démarrage rapide avec bloc de code
  - CTA GitHub + lien vers la documentation

---

## v0.3.0

**Date :** 27 juillet 2026

### Nouvelles fonctionnalités

- **Rate limiting** — Limitation du débit par IP et/ou par table via token bucket (`golang.org/x/time/rate`)
  - Configuration globale (`requests_per_second` + `burst`)
  - Overrides par table dans `rate_limit.per_table`
  - Headers `Retry-After` et `X-RateLimit-Limit` en cas de 429
  - Code d'erreur structuré `PGRST429`
  - Cleanup automatique des limiters inactifs (10 min)
- **IHM index.html** — Route `/` servant une interface web vanilla JS
  - Lien rapide vers `/docs` (Swagger UI)
  - État de santé en temps réel (polling toutes les 5s via `/ops/readiness`)
  - Console SSE interactive avec sélection schema/table et auto-scroll
  - Endpoint `/ops/streams` retournant la map `schema → [tables]` pour peupler les sélecteurs
  - Embed via `go:embed` (pas de fichier externe)
- **Auto notify triggers** — Création automatique des triggers SSE lors du hot-reload
  - Lorsqu'une nouvelle table est détectée, le watcher crée `rest_notify()` + le trigger `{table}_notify`
  - Désactivation possible via `auto_notify_triggers: false` dans la config

### Configuration ajoutée

```yaml
rate_limit:
  enabled: false
  requests_per_second: 10
  burst: 20
  # per_table:
  #   users:
  #     requests_per_second: 5
  #     burst: 10

hot_reload:
  enabled: false
  interval: 30s
  auto_notify_triggers: true
```

### Endpoints ajoutés

| Endpoint | Description |
|----------|-------------|
| `GET /` | IHM index.html |
| `GET /ops/streams` | Map des schemas/tables disponibles pour SSE |

---

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
