CREATE TABLE IF NOT EXISTS audit_logs (
                                          id UUID PRIMARY KEY,
                                          trace_id UUID NOT NULL,
                                          agent_id UUID NOT NULL,
                                          capability_id VARCHAR(255) NOT NULL,

    -- Используем JSONB для эффективного хранения и поиска по данным
    payload JSONB NOT NULL,
    response JSONB,

    mode VARCHAR(20) NOT NULL,    -- 'LIVE', 'SANDBOX'
    status VARCHAR(20) NOT NULL,  -- 'SUCCESS', 'FAILED', 'INTERCEPTED'

    duration_ms INT NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW()
    );

-- Индексы для быстрой фильтрации в Console API
CREATE INDEX idx_audit_agent_timestamp ON audit_logs(agent_id, timestamp DESC);
CREATE INDEX idx_audit_trace_id ON audit_logs(trace_id);

-- GIN-индекс для полнотекстового поиска внутри JSON (если нужно искать по полям payload)
CREATE INDEX idx_audit_payload_gin ON audit_logs USING GIN (payload);
