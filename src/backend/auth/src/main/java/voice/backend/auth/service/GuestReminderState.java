package voice.backend.auth.service;

import java.time.Instant;

/** Guest save-account reminder state (docs/features/auth-and-contacts.md). */
public record GuestReminderState(Instant lastShownAt, boolean shouldShow) {}
