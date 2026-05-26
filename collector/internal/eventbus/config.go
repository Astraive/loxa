package eventbus

// Config holds configuration for creating a Bus instance.
type Config struct {
	// Type selects the adapter: memory, redis, nats, kafka.
	Type string `yaml:"type" json:"type"`

	// Topic is the default topic/subject/stream name.
	Topic string `yaml:"topic" json:"topic"`

	// DLQTopic is the dead-letter topic for failed messages.
	DLQTopic string `yaml:"dlq_topic" json:"dlq_topic"`

	// ConsumerGroup is the consumer group name for subscribe operations.
	ConsumerGroup string `yaml:"consumer_group" json:"consumer_group"`

	// Memory holds memory-adapter config.
	Memory MemoryConfig `yaml:"memory" json:"memory,omitempty"`

	// Redis holds Redis adapter config.
	Redis RedisConfig `yaml:"redis" json:"redis,omitempty"`

	// NATS holds NATS adapter config.
	NATS NATSConfig `yaml:"nats" json:"nats,omitempty"`

	// Kafka holds Kafka adapter config.
	Kafka KafkaConfig `yaml:"kafka" json:"kafka,omitempty"`
}

// MemoryConfig configures the in-process memory adapter.
type MemoryConfig struct {
	// BufferSize is the channel buffer size for the in-process bus.
	BufferSize int `yaml:"buffer_size" json:"buffer_size"`
}

// RedisConfig configures the Redis Streams adapter.
type RedisConfig struct {
	Addr     string `yaml:"addr" json:"addr"`
	Password string `yaml:"password" json:"password"`
	DB       int    `yaml:"db" json:"db"`
	Stream   string `yaml:"stream" json:"stream"`
	Group    string `yaml:"group" json:"group"`
	MaxLen   int64  `yaml:"max_len" json:"max_len"`
}

// NATSConfig configures the NATS JetStream adapter.
type NATSConfig struct {
	URL      string `yaml:"url" json:"url"`
	Stream   string `yaml:"stream" json:"stream"`
	Subject  string `yaml:"subject" json:"subject"`
	Durable  string `yaml:"durable" json:"durable"`
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
}

// KafkaConfig configures the Kafka adapter.
type KafkaConfig struct {
	Brokers          []string `yaml:"brokers" json:"brokers"`
	Topic            string   `yaml:"topic" json:"topic"`
	ConsumerGroup    string   `yaml:"consumer_group" json:"consumer_group"`
	Acks             string   `yaml:"acks" json:"acks"`
	EnableIdempotence bool   `yaml:"enable_idempotence" json:"enable_idempotence"`
	MaxRetries       int      `yaml:"max_retries" json:"max_retries"`
	Compression      string   `yaml:"compression" json:"compression"`
}
