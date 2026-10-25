package voice.backend.auth.principal;

/** Dependency outage is distinct from an invalid or replayed credential. */
final class PrincipalDependencyUnavailableException extends IllegalArgumentException {
  PrincipalDependencyUnavailableException() { super("principal authority unavailable"); }
}
