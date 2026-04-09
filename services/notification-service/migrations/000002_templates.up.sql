CREATE TABLE IF NOT EXISTS notification_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) NOT NULL UNIQUE,
    channel VARCHAR(20) NOT NULL DEFAULT 'email',
    subject VARCHAR(200),
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO notification_templates (name, channel, subject, body) VALUES
('order_confirmed', 'email', 'Order {{.order_id}} confirmed',
 'Your order {{.order_id}} has been confirmed. Awaiting courier assignment.'),
('order_assigned', 'email', 'Courier assigned to order {{.order_id}}',
 'A courier has been assigned to your order {{.order_id}}. Estimated delivery: {{.eta}}'),
('order_delivered', 'email', 'Order {{.order_id}} delivered',
 'Your order {{.order_id}} has been successfully delivered. Thank you!'),
('order_cancelled', 'email', 'Order {{.order_id}} cancelled',
 'Your order {{.order_id}} has been cancelled.')
ON CONFLICT (name) DO NOTHING;
