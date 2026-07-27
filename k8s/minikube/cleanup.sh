#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${1:-rest-trans}"
RELEASE_NAME="${2:-rest-trans}"

echo "=== Nettoyage ==="

echo "Désinstallation du release Helm..."
helm uninstall "$RELEASE_NAME" -n "$NAMESPACE" 2>/dev/null || echo "Release non trouvé"

echo "Suppression de PostgreSQL..."
kubectl delete -f "$(dirname "$0")/manifests/postgres.yaml" -n "$NAMESPACE" 2>/dev/null || echo "Manifests PostgreSQL non trouvés"

echo "Suppression du namespace..."
kubectl delete namespace "$NAMESPACE" 2>/dev/null || echo "Namespace non trouvé"

echo "=== Nettoyage terminé ==="
