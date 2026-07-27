# TODOs

## Haute valeur

- [ ] **Auth JWT** — Middleware pour valider un token JWT et exposer un endpoint `/rpc/auth.user_id()` pour le RLS
- [ ] **RLS (Row Level Security)** — Activer les policies PostgreSQL par schéma, rest-trans les appliquerait automatiquement
- [x] **Hot reload** — Watcher le schema PG pour détecter les changements (nouvelles colonnes, tables) sans restart

## Moyenne valeur

- [x] **Bulk upsert** — Upsert batch avec ON CONFLICT sur n'importe quelles colonnes (pas juste PK)
- [ ] **Full-text search** — Filtre `_fts=col.search_term` avec tsvector/tsquery
- [x] **View support amélioré** — Les vues sont déjà introspectées mais les mutations devraient être bloquées proprement (au lieu du 400 actuel)
- [x] **Metrics/Prometheus** — `/ops/metrics` avec compteurs de requêtes, latence, erreurs
- [x] **Rate limiting** — Configurable par table/IP
- [x] **SSE** — `GET /:schema/:table/_stream` pour du real-time (LISTEN/NOTIFY)
- [x] **IHM index.html** — Route `/` servant un `index.html` avec : liens vers `/docs`, état de santé du composant (`/ops/readiness`), console d'affichage des notifications SSE de la base

## Quick wins

- [x] **Tests avec données isolées** — Chaque test nettoie ses inserts (utiliser des transactions de test)
- [ ] **CI/CD** — GitHub Actions avec `make test` + Docker build
- [x] **Logging structuré** — Passer de `log.Printf` à `slog` ou `zerolog`
- [x] **Graceful shutdown** — Arrêter proprement le serveur et les connexions DB
- [x] **Health check** — `GET /ops/readiness` qui ping la DB et `GET /ops/liveness`
