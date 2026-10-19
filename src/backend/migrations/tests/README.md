# Automod downgrade isolation checks

Run `sh automod-down-isolation.sh /path/to/repository all` inside a disposable
Linux PostgreSQL fixture with PostgreSQL client tools and administrative
`PGUSER`/`PGPASSWORD`. It creates uniquely named databases and drops only databases
it successfully created. No host port or application database is required.

The 22 cases cover seven author-owned downgrade files: pinned REPEATABLE READ
with a subsequently committed first history row; READ COMMITTED with a real
writer-lock wait; unused READ COMMITTED DOWN/UP; and Auth28's existing LIVE
subject at moderation revision0. The checks require the specific isolation or
history refusal, preserved committed history, and successful unused downgrade.
`rr`, `race`, `empty`, and `live` select one group for genuine baseline evidence.

Fixtures apply exact relevant canonical prefixes: Mod1/2/4/6 plus9/13 as needed,
Messaging1..31, and Auth1..28 plus30/31. Auth29 is the separately owned Role-purpose
migration and is not part of this bounded Mod-purpose prefix proof. These checks
do not qualify combined serving behavior, runtime activation, CI, or physical
erasure deadlines. All UP migrations remain unchanged by the correction.
