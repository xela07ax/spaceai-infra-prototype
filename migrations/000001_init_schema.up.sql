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

-- Таблица пользователей Console API
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    username VARCHAR(50) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL, -- Хеш bcrypt
    role VARCHAR(20) NOT NULL DEFAULT 'viewer', -- admin, operator, viewer
    scopes JSONB DEFAULT '[]', -- Детальные права: ["agents:read", "policies:write"]
    last_login TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
    );

-- Индекс для быстрого поиска при логине
CREATE INDEX idx_users_username ON users(username);

-- Пароль 'hydro-super-secret-key-2026-change-me'
INSERT INTO users (username, email, password_hash, role)
VALUES ('admin', 'admin@spaceai.io', '$2a$10$wuM1jVI4ebmjWzheO1tyP.5rGl6LxzBBg5r2v5bEk2KrLc/I3JE.a', 'admin');
