#!/bin/bash
set -e

echo "🚀 Starting DevAI E2E Infrastructure Demo..."

# 1. Сценарий JIRA (LIVE)
echo -e "\n[Step 1] Executing JIRA Delete (LIVE MODE)..."
curl -s -X POST "http://localhost:8080/v1/execute?capability=jira.ticket.delete" \
     -H "X-Agent-ID: admin-agent" \
     -H "Content-Type: application/json" \
     -H "X-DevAI-Token: jira.ticket.delete,slack.message.send,db.query.execute"
     -d '{"ticket_id": "DEV-101"}'

# 2. Включаем Sandbox для Slack-агента через Console
echo -e "\n[Step 2] Enabling SANDBOX MODE for Slack Agent via Console API..."
curl -s -X POST "http://localhost:8000/agents/slack-bot/sandbox?enabled=true"

# 3. Сценарий SLACK (SANDBOX)
echo -e "\n[Step 3] Executing SLACK Message (SANDBOX MODE)..."
curl -s -X POST "http://localhost:8080/v1/execute?capability=slack.message.send" \
     -H "X-Agent-ID: slack-bot" \
     -H "Content-Type: application/json" \
     -H "X-DevAI-Token: jira.ticket.delete,slack.message.send,db.query.execute" \
     -d '{"text": "Hello Team!"}'

# 4. Блокируем Slack-агента (Kill-Switch)
echo -e "\n[Step 4] Triggering KILL-SWITCH for Slack Agent..."
curl -s -X POST "http://localhost:8000/agents/slack-bot/block"

# 5. Проверка блокировки
echo -e "\n[Step 5] Attempting action with BLOCKED Agent..."
curl -v -X POST "http://localhost:8080/v1/execute?capability=slack.message.send" \
     -H "X-DevAI-Token: jira.ticket.delete,slack.message.send,db.query.execute" \
     -H "X-Agent-ID: slack-bot" 2>&1 | grep "HTTP/1.1 403"


# Сценарий: Rate Limiting
echo -e "\n[Step 6] Testing Rate Limiting (Spamming requests)..."
for i in {1..10}; do
  curl -s -o /dev/null -w "%{http_code} " -X POST "http://localhost:8080/v1/execute?capability=jira.ticket.delete" \
       -H "X-Agent-ID: admin-agent" \
       -H "X-DevAI-Token: jira.ticket.delete,slack.message.send,db.query.execute"
done

# Сценарий: Circuit Breaker
echo -e "\n\n[Step 7] Testing Circuit Breaker (Unstable service)..."
for i in {1..6}; do
  curl -s -o /dev/null -w "%{http_code} " -X POST "http://localhost:8080/v1/execute?capability=unstable.service" \
       -H "X-Agent-ID: admin-agent" \
       -H "X-DevAI-Token: jira.ticket.delete,slack.message.send,db.query.execute,unstable.service"
done

echo -e "\n(Last 503/500 means Circuit Breaker is OPEN)"

# Сценарий: Database (Internal System)
echo -e "\n[Step 8] Executing DB Query (Internal System)..."
curl -s -X POST "http://localhost:8080/v1/execute?capability=db.query.execute" \
     -H "X-Agent-ID: data-analyst-agent" \
     -H "X-DevAI-Token: jira.ticket.delete,slack.message.send,db.query.execute,unstable.service" \
     -d '{"query": "SELECT * FROM balances"}' | jq

echo -e "\n✅ Demo finished. Check SQL scripts for evidence."
