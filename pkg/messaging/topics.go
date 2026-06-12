package messaging

// Kafka topics for the message write pipeline (03-message-pipeline §3.2).
// Partition key is always conversation_id — the physical basis of per-conversation
// ordering. Versioned names: payload contract changes require a new topic version.
const (
	// TopicToTransfer carries message.submitted events from msg-rpc to msgtransfer
	// (seq not yet allocated).
	TopicToTransfer = "msg.toTransfer.v1"
	// TopicToPostgres carries seq-assigned message.accepted events from the
	// msgtransfer hot path to its async PostgreSQL persist consumer.
	TopicToPostgres = "msg.toPostgres.v1"
	// TopicToPush carries seq-assigned message.accepted events to the push fanout
	// consumer (in-process gateway dispatch until 03 §9 C2 splits service/push).
	TopicToPush = "msg.toPush.v1"
	// TopicAgentTrigger carries message.accepted events that may trigger AI hosting.
	// Consumed by msg-rpc's hosting runtime for now; moves to the agent domain with
	// 04-agent.md. Hosting/recursion filtering stays on the consumer side to keep
	// msgtransfer free of agent-domain logic.
	TopicAgentTrigger = "agent.trigger.v1"
)

const (
	GroupTransfer = "msgtransfer"
	GroupPersist  = "msgtransfer-postgres"
	GroupPush     = "msgtransfer-push"
	// GroupAgentTrigger is the transitional consumer group inside msg-rpc (B2
	// 回流): retires at D15 migration step ④ once GroupAgentService owns the topic.
	GroupAgentTrigger = "msg-rpc-agent-trigger"
	// GroupAgentService is service/agent's consumer group on agent.trigger.v1
	// (D15 final judgment lives there; 04-agent §4.2 step ③).
	GroupAgentService = "agent-trigger"
)
