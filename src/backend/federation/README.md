# Federation Service

Go scaffold for the Voice federation service.

Current public surface:

- GET /health returns {"service":"federation","status":"ok"}.

Domain behavior, gRPC handlers, database repositories, and migrations are intentionally out of scope for this initialization step.