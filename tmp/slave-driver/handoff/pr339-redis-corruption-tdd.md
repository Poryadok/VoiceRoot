# T-339 Redis OTP resend corruption — RED plan

## Contract

`docs/microservices/auth-service.md` states that Redis error, corrupt Redis state, and an impossible Redis response close the OTP flow with `auth_unavailable`; only a valid, current resend reservation may produce `otp_rate_limited`. Redis details must not be exposed.

## RED tests added

1. Mocked resend Lua execution returning `null` or an impossible numeric reply (`2`) must throw `AuthException("auth_unavailable")`.
2. Real Redis resend keys that are corrupt (`not-a-reservation`, even with TTL) or lack TTL (`"1"` without expiry) must throw `AuthException("auth_unavailable")`.

## Green handoff

Keep `0` exclusively for a valid existing resend reservation. Change only `RedisOtpThrottle`/its Lua admission handling as needed to validate the existing marker and its positive TTL before classifying it as rate-limited. Preserve the atomic `SET NX PX` reservation and do not expose Redis details. Run `cd src/backend/auth; mvn -B test` after implementation.
