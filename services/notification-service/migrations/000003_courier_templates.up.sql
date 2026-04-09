INSERT INTO notification_templates (name, channel, subject, body) VALUES
('courier_assigned', 'email', 'New delivery assigned: {{.order_id}}',
 'You have been assigned to deliver order {{.order_id}} (delivery {{.delivery_id}}). Estimated delivery time: {{.eta}}.')
ON CONFLICT (name) DO NOTHING;
