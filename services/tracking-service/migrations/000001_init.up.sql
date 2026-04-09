CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE IF NOT EXISTS location_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    courier_id UUID NOT NULL,
    order_id UUID NOT NULL,
    coordinates GEOMETRY(Point, 4326) NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_location_history_courier ON location_history(courier_id);
CREATE INDEX idx_location_history_order ON location_history(order_id);
CREATE INDEX idx_location_history_time ON location_history(recorded_at DESC);
CREATE INDEX idx_location_history_coordinates ON location_history USING GIST(coordinates);
