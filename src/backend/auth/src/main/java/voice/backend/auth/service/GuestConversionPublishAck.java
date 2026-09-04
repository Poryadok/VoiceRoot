package voice.backend.auth.service;

/** Durable JetStream acknowledgement for a guest-conversion event. */
public record GuestConversionPublishAck(String stream, long sequence) {}
