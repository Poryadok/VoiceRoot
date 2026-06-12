package push

// Payload is the platform-neutral push envelope shared by FCM and APNs senders.
type Payload struct {
	Title       string
	Body        string
	CollapseTag string
	Counter     int
	Data        map[string]string
}
