# Social Service

Go service owning the social graph in PostgreSQL `social_db`: friend invitations,
accepted friendships, account-level blocks, contacts/favourites and phone-hash
resolution. It exposes gRPC `SocialService`, publishes `social.events` through
NATS, and is reached by the Gateway under `/api/v1/friends/**`.

Friend invitations enforce the target profile's `allow_friend_requests` privacy
audience before persistence; a denied request returns `PermissionDenied` and
creates no pending row. The service also uses the documented authenticated S2S
principal boundary for User/Space privacy inputs. Canonical API and operational
details: [`docs/microservices/social-service.md`](../../../docs/microservices/social-service.md).
