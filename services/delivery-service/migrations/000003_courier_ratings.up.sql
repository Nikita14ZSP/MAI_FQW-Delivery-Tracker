CREATE TABLE courier_ratings (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    delivery_id UUID NOT NULL REFERENCES deliveries(id),
    courier_id  UUID NOT NULL,
    user_id     UUID NOT NULL,
    stars       SMALLINT NOT NULL CHECK (stars BETWEEN 1 AND 5),
    comment     TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (delivery_id)
);
CREATE INDEX idx_courier_ratings_courier_id ON courier_ratings(courier_id);
