# TODOs

## Haute valeur

- [ ] **Auth JWT** — Middleware pour valider un token JWT et exposer un endpoint `/rpc/auth.user_id()` pour le RLS
- [ ] **RLS (Row Level Security)** — Activer les policies PostgreSQL par schéma, rest-trans les appliquerait automatiquement
- [ ] **Hot reload** — Watcher le schema PG pour détecter les changements (nouvelles colonnes, tables) sans restart

## Moyenne valeur

- [ ] **Bulk upsert** — Upsert batch avec ON CONFLICT sur n'importe quelles colonnes (pas juste PK)
- [ ] **Full-text search** — Filtre `_fts=col.search_term` avec tsvector/tsquery
- [ ] **View support amélioré** — Les vues sont déjà introspectées mais les mutations devraient être bloquées proprement (au lieu du 400 actuel)
- [ ] **Metrics/Prometheus** — `/ops/metrics` avec compteurs de requêtes, latence, erreurs
- [ ] **Rate limiting** — Configurable par table/IP
- [ ] **SSE** — `GET /:schema/:table?_stream=true` pour du real-time (LISTEN/NOTIFY)

## Quick wins

- [ ] **Tests avec données isolées** — Chaque test nettoie ses inserts (utiliser des transactions de test)
- [ ] **CI/CD** — GitHub Actions avec `make test` + Docker build
- [ ] **Logging structuré** — Passer de `log.Printf` à `slog` ou `zerolog`
- [ ] **Graceful shutdown** — Arrêter proprement le serveur et les connexions DB
- [ ] **Health check** — `GET /ops/health` qui ping la DB
