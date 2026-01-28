#!/bin/bash
# 1. Проверяем работу агента (должен пройти)
echo "--- Testing Live Agent ---"
curl -X POST "http://localhost:8080/v1/execute?capability=crm.read" \
     -H "X-Agent-ID: agent-007" \
     -H "Content-Type: application/json" \
     -d '{"action": "get_user", "id": 1}'

# 2. Блокируем агента через Console
echo -e "\n\n--- Blocking Agent via Console ---"
curl -X POST "http://localhost:8000/agents/agent-007/block"

# 3. Проверяем работу агента снова (должен быть 403 Forbidden)
echo -e "\n--- Testing Blocked Agent ---"
curl -I -X POST "http://localhost:8080/v1/execute?capability=crm.read" \
     -H "X-Agent-ID: agent-007"
