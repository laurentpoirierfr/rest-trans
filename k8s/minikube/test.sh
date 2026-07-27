#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${1:-rest-trans}"
SERVICE_NAME="${2:-rest-trans}"
MAX_RETRIES=30
RETRY_INTERVAL=2

echo "=== Test du déploiement ==="

echo ""
echo "--- Vérification de PostgreSQL ---"
for i in $(seq 1 $MAX_RETRIES); do
    PG_READY=$(kubectl get pods -n "$NAMESPACE" -l app=postgres \
        -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "")
    
    if [ "$PG_READY" = "True" ]; then
        echo "✓ PostgreSQL prêt"
        break
    else
        echo "PostgreSQL en cours de démarrage... ($i/$MAX_RETRIES)"
        sleep $RETRY_INTERVAL
    fi
done

echo ""
echo "--- Vérification de rest-trans ---"
for i in $(seq 1 $MAX_RETRIES); do
    RT_READY=$(kubectl get pods -n "$NAMESPACE" -l "app.kubernetes.io/name=rest-trans" \
        -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "")
    
    if [ "$RT_READY" = "True" ]; then
        echo "✓ rest-trans prêt"
        break
    else
        echo "rest-trans en cours de démarrage... ($i/$MAX_RETRIES)"
        sleep $RETRY_INTERVAL
    fi
done

echo ""
echo "--- État des ressources ---"
kubectl get pods -n "$NAMESPACE" -o wide
echo ""
kubectl get svc -n "$NAMESPACE"

echo ""
echo "--- Logs de rest-trans ---"
POD=$(kubectl get pods -n "$NAMESPACE" -l "app.kubernetes.io/name=rest-trans" \
    -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

if [ -n "$POD" ]; then
    kubectl logs -n "$NAMESPACE" "$POD" --tail=20
fi

echo ""
echo "--- Test de connectivité ---"
if kubectl get svc -n "$NAMESPACE" "$SERVICE_NAME" &>/dev/null; then
    echo "Service $SERVICE_NAME trouvé"
    
    kubectl port-forward -n "$NAMESPACE" "svc/$SERVICE_NAME" 18080:3000 &
    PF_PID=$!
    sleep 3
    
    if curl -s --max-time 5 http://localhost:18080/info >/dev/null 2>&1; then
        echo "✓ API accessible via port-forward"
        echo ""
        echo "Réponse /info:"
        curl -s http://localhost:18080/info | python3 -m json.tool 2>/dev/null || curl -s http://localhost:18080/info
    else
        echo "✗ API non accessible (le pod est peut-être en cours de démarrage)"
    fi
    
    echo ""
    kill $PF_PID 2>/dev/null || true
else
    echo "✗ Service $SERVICE_NAME non trouvé"
fi

echo ""
echo "--- Commandes utiles ---"
echo "Port-forward:  kubectl port-forward -n $NAMESPACE svc/$SERVICE_NAME 3000:3000"
echo "Logs:          kubectl logs -n $NAMESPACE -l app.kubernetes.io/name=rest-trans -f"
echo "Shell pod:     kubectl exec -it -n $NAMESPACE deploy/$SERVICE_NAME -- /bin/sh"
