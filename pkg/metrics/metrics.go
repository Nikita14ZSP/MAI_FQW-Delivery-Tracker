package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// OrdersCreatedTotal counts the total number of orders created.
	OrdersCreatedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "orders_created_total",
		Help: "Total number of orders created",
	})

	// DeliveriesActive tracks the current number of active deliveries.
	DeliveriesActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "deliveries_active",
		Help: "Current number of active deliveries",
	})

	// WebsocketConnectionsActive tracks the current number of active WebSocket connections.
	WebsocketConnectionsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "websocket_connections_active",
		Help: "Current number of active WebSocket connections",
	})

	// KafkaConsumerLag tracks Kafka consumer lag by topic and consumer group.
	KafkaConsumerLag = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "kafka_consumer_lag",
		Help: "Kafka consumer lag by topic and group",
	}, []string{"topic", "group"})
)
