CREATE TABLE IF NOT EXISTS approvals (
                                         id UUID PRIMARY KEY,
                                         execution_id UUID NOT NULL,
                                         agent_id UUID NOT NULL,
                                         capability VARCHAR(255) NOT NULL,
    payload TEXT,
    status VARCHAR(20) DEFAULT 'PENDING',
    reviewer_id UUID,
    comment TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
    );
CREATE INDEX idx_approvals_pending ON approvals(status) WHERE status = 'PENDING';
