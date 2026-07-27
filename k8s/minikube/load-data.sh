#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DATA_DIR="$SCRIPT_DIR/../../tests/data"

NAMESPACE="${1:-rest-trans}"
SERVICE_NAME="${2:-rest-trans}"
LOCAL_PORT="${3:-18080}"

echo "=== Chargement des données de test ==="

echo ""
echo "--- Démarrage du port-forward ---"
kubectl port-forward -n "$NAMESPACE" "svc/$SERVICE_NAME" "$LOCAL_PORT:3000" &
PF_PID=$!
trap "kill $PF_PID 2>/dev/null" EXIT

echo "Attente du port-forward..."
sleep 3

echo ""
echo "--- Vérification de l'API ---"
if ! curl -s --max-time 5 "http://localhost:$LOCAL_PORT/info" >/dev/null 2>&1; then
    echo "✗ API non accessible"
    exit 1
fi
echo "✓ API accessible"

echo ""
echo "--- Chargement des utilisateurs ---"
if [ -f "$DATA_DIR/users.json" ]; then
    RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "http://localhost:$LOCAL_PORT/public/users" \
        -H "Content-Type: application/json" \
        -d @"$DATA_DIR/users.json")
    HTTP_CODE=$(echo "$RESPONSE" | tail -1)
    BODY=$(echo "$RESPONSE" | head -n -1)
    if [ "$HTTP_CODE" = "201" ]; then
        echo "✓ Utilisateurs chargés"
    else
        echo "✗ Erreur ($HTTP_CODE): $BODY"
    fi
else
    echo "⚠ Fichier users.json non trouvé"
fi

echo ""
echo "--- Chargement des projets ---"
if [ -f "$DATA_DIR/projects.json" ]; then
    RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "http://localhost:$LOCAL_PORT/public/projects" \
        -H "Content-Type: application/json" \
        -d @"$DATA_DIR/projects.json")
    HTTP_CODE=$(echo "$RESPONSE" | tail -1)
    BODY=$(echo "$RESPONSE" | head -n -1)
    if [ "$HTTP_CODE" = "201" ]; then
        echo "✓ Projets chargés"
    else
        echo "✗ Erreur ($HTTP_CODE): $BODY"
    fi
else
    echo "⚠ Fichier projects.json non trouvé"
fi

echo ""
echo "--- Chargement des tâches ---"
if [ -f "$DATA_DIR/tasks.json" ]; then
    RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "http://localhost:$LOCAL_PORT/public/project_tasks" \
        -H "Content-Type: application/json" \
        -d @"$DATA_DIR/tasks.json")
    HTTP_CODE=$(echo "$RESPONSE" | tail -1)
    BODY=$(echo "$RESPONSE" | head -n -1)
    if [ "$HTTP_CODE" = "201" ]; then
        echo "✓ Tâches chargées"
    else
        echo "✗ Erreur ($HTTP_CODE): $BODY"
    fi
else
    echo "⚠ Fichier tasks.json non trouvé"
fi

echo ""
echo "--- Vérification ---"
echo "Utilisateurs:"
curl -s "http://localhost:$LOCAL_PORT/public/users?_select=id,name,email" | python3 -m json.tool 2>/dev/null || curl -s "http://localhost:$LOCAL_PORT/public/users?_select=id,name,email"
echo ""
echo "Projets:"
curl -s "http://localhost:$LOCAL_PORT/public/projects?_select=id,title,status" | python3 -m json.tool 2>/dev/null || curl -s "http://localhost:$LOCAL_PORT/public/projects?_select=id,title,status"
echo ""
echo "Tâches:"
curl -s "http://localhost:$LOCAL_PORT/public/project_tasks?_select=id,project_id,title" | python3 -m json.tool 2>/dev/null || curl -s "http://localhost:$LOCAL_PORT/public/project_tasks?_select=id,project_id,title"
echo ""
echo "Vue active_users:"
curl -s "http://localhost:$LOCAL_PORT/public/active_users?_select=id,name,email" | python3 -m json.tool 2>/dev/null || curl -s "http://localhost:$LOCAL_PORT/public/active_users?_select=id,name,email"

echo ""
echo "=== Données chargées ==="
