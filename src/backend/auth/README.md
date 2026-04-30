# Auth Service

Java/Spring Boot scaffold for registration, login, JWT, 2FA, and guest accounts.

Current public surface:

- `GET /health` returns `{"service":"auth","status":"ok"}`.
- Spring Actuator is present for future operational probes.

Domain behavior, database repositories, Flyway migrations, and real auth endpoints are intentionally out of scope for this initialization step.
