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
	// TopicToPush carries seq-assigned message.accepted events to the push fanout.
	// Consumed by service/push (03 §9 C2): online broadcast to all gateways.
	TopicToPush = "msg.toPush.v1"
	// TopicToOfflinePush carries the second-stage offline fan-out (03 §6.3 / D5):
	// service/push produces it for recipients online delivery missed, and its own
	// offline consumer re-reads it to drive the vendor pusher (FCM/APNs/…).
	TopicToOfflinePush = "msg.toOfflinePush.v1"
	// TopicAgentTrigger carries message.accepted events that may trigger AI hosting.
	// Consumed by msg-rpc's hosting runtime for now; moves to the agent domain with
	// 04-agent.md. Hosting/recursion filtering stays on the consumer side to keep
	// msgtransfer free of agent-domain logic.
	TopicAgentTrigger = "agent.trigger.v1"
)

const (
	GroupTransfer = "msgtransfer"
	GroupPersist  = "msgtransfer-postgres"
	// GroupPushOnline consumes msg.toPush.v1 in service/push (D5): online broadcast.
	GroupPushOnline = "push-online"
	// GroupPushOffline consumes msg.toOfflinePush.v1 in service/push (D5): vendor push.
	GroupPushOffline = "push-offline"
	// GroupAgentTrigger is the transitional consumer group inside msg-rpc (B2
	// 回流): retires at D15 migration step ④ once GroupAgentService owns the topic.
	GroupAgentTrigger = "msg-rpc-agent-trigger"
	// GroupAgentService is service/agent's consumer group on agent.trigger.v1
	// (D15 final judgment lives there; 04-agent §4.2 step ③).
	GroupAgentService = "agent-trigger"
)
