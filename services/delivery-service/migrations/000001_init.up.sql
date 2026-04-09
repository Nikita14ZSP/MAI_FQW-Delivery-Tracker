CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE IF NOT EXISTS deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL,
    courier_id UUID,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    zone_id UUID,
    estimated_delivery TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS delivery_zones (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    boundary GEOMETRY(Polygon, 4326) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS courier_zones (
    courier_id UUID NOT NULL,
    zone_id UUID NOT NULL REFERENCES delivery_zones(id),
    PRIMARY KEY (courier_id, zone_id)
);

CREATE INDEX idx_deliveries_order_id ON deliveries(order_id);
CREATE INDEX idx_deliveries_courier_id ON deliveries(courier_id);
CREATE INDEX idx_delivery_zones_boundary ON delivery_zones USING GIST(boundary);
