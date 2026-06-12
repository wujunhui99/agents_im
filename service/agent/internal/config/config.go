package config

// Config is the service/agent trigger-consumer configuration. The service is
// interface-first scaffolding (issue #503): every Driver field currently only
// accepts "mock" — real drivers (LLM runtime, msg-rpc write-back, goctl
// hosting model) land with 04-agent.md §5 and must be wired here explicitly.
type Config struct {
	Name  string `json:",default=agent"`
	Kafka KafkaConf
	// Runtime selects the agent run executor. mock = canned reply, no LLM.
	Runtime DriverConf
	// Sender selects the IM write-back adapter. mock = log only, nothing is
	// written back to msg-rpc (keeps this consumer side-effect free while the
	// transitional msg-rpc 回流 consumer still owns real AI replies).
	Sender DriverConf
	// Hosting selects the conversation_ai_hosting lookup. mock = static map.
	Hosting DriverConf
}

type KafkaConf struct {
	// Brokers is a comma-separated bootstrap list; env KAFKA_BROKERS overrides.
	Brokers string `json:",optional"`
	// Group is the consumer group on agent.trigger.v1 — distinct from the
	// transitional msg-rpc group so both can run side by side until D15 step ④.
	Group string `json:",default=agent-trigger"`
}

type DriverConf struct {
	Driver string `json:",default=mock"`
}

const DriverMock = "mock"
