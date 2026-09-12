# Role Service

Go microservice: space roles, permission masks, and permanent Space retirement. gRPC `RoleService`. PostgreSQL `role_db`. Gateway `/api/v1/roles/**`; authenticated Space workload calls, including `RetireSpace`, use the dedicated TLS listener. See `docs/microservices/role-service.md`.
