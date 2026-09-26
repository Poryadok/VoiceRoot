# Realtime JetStream ownership

PR1 makes Realtime a bind-only JetStream consumer. The bootstrap job creates
the listed streams and fixed, filtered durable push consumers before Realtime
starts. Realtime has no stream or consumer administration path.

| Stream | Realtime durable | Filter subject | Delivery subject |
| --- | --- | --- | --- |
| `message_events` | `rt_realtime1_msg` | `message.>` | `_INBOX.voice.realtime1.message` |
| `chat_events` | `rt_realtime1_chat` | `chat.>` | `_INBOX.voice.realtime1.chat` |
| `user_events` | `rt_realtime1_user` | `user.presence_changed` | `_INBOX.voice.realtime1.user` |
| `social_events` | `rt_realtime1_social` | `social.user_blocked` | `_INBOX.voice.realtime1.social` |
| `role_events` | `rt_realtime1_role` | `role.>` | `_INBOX.voice.realtime1.role` |
| `voice_events` | `rt_realtime1_voice` | `voice.>` | `_INBOX.voice.realtime1.voice` |
| `matchmaking_events` | `rt_realtime1_matchmaking` | `mm.>` | `_INBOX.voice.realtime1.matchmaking` |

The sole deployed slot is `realtime-1`. Its Deployment has `replicas: 1` and
`Recreate` rollout strategy: scaling it or changing the slot name requires a
new explicitly pre-provisioned consumer and a matching authorization review.
Missing consumers are a startup error; Realtime must not create one.

Staging Notification also uses `Recreate`: its fixed push durables have no
queue group, so old and new replicas cannot bind concurrently. The staging
manifest apply first removes Kubernetes' defaulted `rollingUpdate` field with a
strategic retain-keys patch before applying the canonical strategy.

The bootstrap entry points are `docker/nats/realtime-bootstrap.sh` for Compose
and the `voice-nats-realtime-bootstrap` Job in
`deploy/templates/nats-realtime-bootstrap.yaml` for staging and production.
The current job connects without credentials only until the JWT activation PR;
that activation will mount a dedicated bootstrap credential only into the Job.

## JWT activation dependency

JWT activation must grant Realtime only the exact Core delivery subjects and
the exact JetStream bind/fetch/ack API subjects required by these consumers.
It must not grant `$JS.API.>` or stream/consumer create, update, or delete.
The activation PR must include a negative proof that a Realtime credential
cannot read another service's consumer or administer a stream.
