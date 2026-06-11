package delivery

// ApplySettings filters delivery by mute/suppress rules. Mentions override chat mute when configured.
func ApplySettings(base DeliveryDecision, in DeliveryInput, settings SettingsSnapshot) DeliveryDecision {
	out := base
	if settings.ChatMuted && in.Type == TypeNewMessage {
		return DeliveryDecision{}
	}
	if settings.ChatMuted && in.Type == TypeMention && settings.MentionOverridesMute {
		return DeliveryDecision{InApp: true, Push: true}
	}
	for _, suppressed := range settings.SuppressTypes {
		if suppressed == in.Type {
			out.Push = false
			out.InApp = false
			break
		}
	}
	return out
}
