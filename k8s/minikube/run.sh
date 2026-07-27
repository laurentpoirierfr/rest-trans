#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

NAMESPACE="${1:-rest-trans}"
REPLICAS="${2:-2}"

echo "==========================================="
echo "  Test complet Minikube - rest-trans"
echo "==========================================="
echo ""

echo "Étape 1/5: Démarrage de Minikube"
bash "$SCRIPT_DIR/start.sh"
echo ""

echo "Étape 2/5: Déploiement"
bash "$SCRIPT_DIR/deploy.sh" "$NAMESPACE" "rest-trans" "$REPLICAS"
echo ""

echo "Étape 3/5: Chargement des données"
bash "$SCRIPT_DIR/load-data.sh" "$NAMESPACE"
echo ""

echo "Étape 4/5: Tests"
bash "$SCRIPT_DIR/test.sh" "$NAMESPACE"
echo ""

echo "==========================================="
echo "  Déploiement réussi!"
echo "==========================================="
echo ""
echo "Pour nettoyer: bash $SCRIPT_DIR/cleanup.sh $NAMESPACE"
