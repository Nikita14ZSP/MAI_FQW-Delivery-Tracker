CREATE TABLE IF NOT EXISTS couriers (
    id UUID PRIMARY KEY,
    status VARCHAR(20) NOT NULL DEFAULT 'available',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_couriers_status ON couriers(status);
