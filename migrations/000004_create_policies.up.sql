CREATE TABLE IF NOT EXISTS policies (
                                        id UUID PRIMARY KEY,
                                        agent_id VARCHAR(255) NOT NULL, -- UUID или '*'
    capability_id VARCHAR(255) NOT NULL,
    effect VARCHAR(20) NOT NULL,    -- ALLOW, DENY, SANDBOX, QUARANTINE
    conditions JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
    );

CREATE INDEX idx_policies_lookup ON policies(agent_id, capability_id);

/* Для демонстрации!: если в базе будет пример с Wildcard-субъектов ("*") в системе политик.
   Это позволяет гибко управлять доступом: мы можем запретить db.delete для всех через одну запись (*, db.delete, DENY),
   но разрешить её для конкретного административного агента.
 */

-- Глобальная политика: запрещаем Slack всем агентам по умолчанию (Safety First)
INSERT INTO policies (id, agent_id, capability_id, effect, conditions)
VALUES (gen_random_uuid(), '*', 'slack.message.send', 'DENY', '{"reason": "global security limit"}');

-- Исключение: конкретному агенту (например, HR-Agent) разрешаем Slack
INSERT INTO policies (id, agent_id, capability_id, effect, conditions)
VALUES (gen_random_uuid(), 'hr-agent-uuid', 'slack.message.send', 'ALLOW', '{}');

-- «Разрешай переводы, но если поле amount больше 5000 — зови человека
INSERT INTO policies (id, agent_id, capability_id, effect, conditions)
VALUES (
           gen_random_uuid(),
           'finance-agent-001',
           'bank.transfer.execute',
           'ALLOW',
           '{
               "risk_field": "amount",
               "threshold": 5000.0,
               "description": "Требовать апрув для крупных транзакций"
           }'
       );
