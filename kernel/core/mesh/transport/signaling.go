package transport

// SignalingChannel defines the interface for the signaling server connection
type SignalingChannel interface {
	Send(message interface{}) error
	Receive() ([]byte, error)
	Close() error
	IsConnected() bool
}
