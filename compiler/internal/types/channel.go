package types

// Channel represents a thread-safe message passing channel.
// Usage: Channel[T].new(buffer=N) to create
type Channel struct {
	Elem T // The element type being sent/received
}

func (*Channel) isType() {}
func (c *Channel) String() string {
	return "Channel[" + c.Elem.String() + "]"
}

// ChannelSender represents the sending half of a channel.
// Multiple senders can exist (MPSC pattern).
type ChannelSender struct {
	Elem T // The element type
}

func (*ChannelSender) isType() {}
func (s *ChannelSender) String() string {
	return "Sender[" + s.Elem.String() + "]"
}

// ChannelReceiver represents the receiving half of a channel.
// Typically only one receiver exists (single consumer).
type ChannelReceiver struct {
	Elem T // The element type
}

func (*ChannelReceiver) isType() {}
func (r *ChannelReceiver) String() string {
	return "Receiver[" + r.Elem.String() + "]"
}

// ChannelOf creates a Channel type for the given element type
func ChannelOf(elem T) *Channel {
	return &Channel{Elem: elem}
}

// SenderOf creates a ChannelSender type
func SenderOf(elem T) *ChannelSender {
	return &ChannelSender{Elem: elem}
}

// ReceiverOf creates a ChannelReceiver type
func ReceiverOf(elem T) *ChannelReceiver {
	return &ChannelReceiver{Elem: elem}
}
