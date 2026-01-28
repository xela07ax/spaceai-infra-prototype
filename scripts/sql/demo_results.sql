-- ПОКАЗАТЬ РЕЗУЛЬТАТЫ ДЕМО: JIRA vs SLACK + Sandbox + KillSwitch
SELECT
    agent_id,
    capability_id,
    mode,
    status,
    duration_ms || 'ms' as latency,
    CASE
        WHEN status = 'BLOCKED' THEN '⛔ Security Blocked'
        WHEN mode = 'SANDBOX' THEN '🧪 Virtual Action'
        ELSE '✅ Real Execution'
        END as engineering_note,
    timestamp
FROM audit_logs
ORDER BY timestamp DESC
    LIMIT 5;
