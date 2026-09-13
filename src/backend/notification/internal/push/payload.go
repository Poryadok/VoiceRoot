package push

// Payload is the platform-neutral push envelope shared by FCM and APNs senders.
type Payload struct {
	Title       string
	Body        string
	CollapseTag string
	Counter     int
	// Silent delivers a visible push without sound or a badge increment where
	// the platform supports those controls. It does not affect in-app routing.
	Silent bool
	Data   map[string]string
}
