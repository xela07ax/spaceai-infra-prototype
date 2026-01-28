-- =========================================================================
-- DEBUG: Анализ действий агентов через AgentFS (audit_logs)
-- =========================================================================

-- 1. Сквозная трассировка по Trace-ID
-- Позволяет увидеть всю цепочку событий одного запроса
SELECT * FROM audit_logs WHERE trace_id = 'YOUR-TRACE-ID-HERE';

-- 2. Проверка работы Sandbox
-- Ищем действия, которые были перехвачены (INTERCEPTED)
SELECT agent_id, capability_id, payload, response, timestamp
FROM audit_logs
WHERE mode = 'SANDBOX'
ORDER BY timestamp DESC;

-- 3. Аналитика ошибок исполнения
-- Помогает понять, какие коннекторы падают (FAILED)
SELECT capability_id, error, COUNT(*)
FROM audit_logs
WHERE status = 'FAILED'
GROUP BY capability_id, error;
