#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
HELM_DIR="$PROJECT_DIR/k8s/helm"
MANIFESTS_DIR="$SCRIPT_DIR/manifests"

NAMESPACE="${1:-rest-trans}"
RELEASE_NAME="${2:-rest-trans}"
REPLICAS="${3:-2}"

echo "=== Déploiement dans le namespace: $NAMESPACE ==="

if ! command -v helm &>/dev/null; then
    echo "Erreur: helm n'est pas installé"
    exit 1
fi

if ! kubectl get namespace "$NAMESPACE" &>/dev/null; then
    echo "Création du namespace $NAMESPACE..."
    kubectl create namespace "$NAMESPACE"
fi

echo ""
echo "=== Étape 1/2: Déploiement de PostgreSQL ==="
kubectl apply -f "$MANIFESTS_DIR/postgres.yaml" -n "$NAMESPACE"

echo "Attente que PostgreSQL soit prêt..."
kubectl wait --for=condition=ready pod -l app=postgres -n "$NAMESPACE" --timeout=120s
echo "✓ PostgreSQL prêt"

echo ""
echo "=== Étape 2/2: Déploiement de rest-trans ==="
echo "Installation du chart Helm..."

helm upgrade --install "$RELEASE_NAME" "$HELM_DIR" \
    --namespace "$NAMESPACE" \
    --set replicaCount="$REPLICAS" \
    --set image.repository=rest-trans \
    --set image.tag=latest \
    --set image.pullPolicy=Never \
    --set database.host=postgres \
    --set database.port=5432 \
    --set database.user=postgres \
    --set database.password=postgres \
    --set database.name=app

echo ""
echo "Attente que rest-trans soit prêt..."
kubectl wait --for=condition=ready pod -l "app.kubernetes.io/name=rest-trans" -n "$NAMESPACE" --timeout=120s || {
    echo "⚠ Timeout - affichage des logs:"
    kubectl logs -n "$NAMESPACE" -l "app.kubernetes.io/name=rest-trans" --tail=30 || true
    kubectl get pods -n "$NAMESPACE"
    exit 1
}

echo ""
echo "=== Déploiement terminé ==="
echo ""
echo "Pods:"
kubectl get pods -n "$NAMESPACE"
echo ""
echo "Services:"
kubectl get svc -n "$NAMESPACE"
