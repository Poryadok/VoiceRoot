package voice.backend.auth.repository;

/** Raised when an active provider source is already bound to another profile. */
public final class LinkedIdentityProfileConflictException extends RuntimeException {}
