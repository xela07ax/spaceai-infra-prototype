CREATE TABLE IF NOT EXISTS agents (
                                      id UUID PRIMARY KEY,
                                      name VARCHAR(255) NOT NULL,
    status VARCHAR(50) DEFAULT 'active', -- active, blocked, quarantine
    is_sandbox BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
    );

-- Индекс для быстрого поиска по статусу (нужен для аналитики в Console)
CREATE INDEX idx_agents_status ON agents(status);
