#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "=== Démarrage Minikube ==="

if ! command -v minikube &>/dev/null; then
    echo "Erreur: minikube n'est pas installé"
    exit 1
fi

if minikube status --format='{{.Host}}' 2>/dev/null | grep -q "Running"; then
    echo "Minikube est déjà en cours d'exécution"
else
    minikube start --driver=docker --extra-config=apiserver.enable-admission-plugins="LimitRanger,NamespaceExists,NamespaceLifecycle,ResourceQuota,ServiceAccount,DefaultStorageClass,MutatingAdmissionWebhook"
fi

echo "=== Attente de l'API server ==="
for i in $(seq 1 60); do
    if kubectl cluster-info &>/dev/null 2>&1; then
        echo "✓ API server prêt"
        break
    fi
    echo "En attente de l'API server... ($i/60)"
    sleep 2
done

echo "=== Activation des addons ==="
minikube addons enable ingress || echo "⚠ Ingress non activé"
minikube addons enable metrics-server || echo "⚠ Metrics-server non activé"

echo "=== Configuration Docker ==="
eval $(minikube docker-env)

echo "=== Build de l'image Docker ==="
cd "$PROJECT_DIR"
docker build -t rest-trans:latest .

echo "=== Minikube prêt ==="
minikube status
