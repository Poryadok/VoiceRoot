# Guarded correction provenance, version 1

This is a finite producer-provenance capability under SWARM-DEVELOP-20261018
c23-c24. It qualifies an independently reviewed correction of an already owned
guarded path; it grants no R22 runtime activation, path exemption, deployment or
release approval. Existing Game checkpoint and release rules remain unchanged.

The machine record is `guarded-correction-voice013-v1.json` beside this document.
Its carrier commit R must descend from correction C. A later reviewed verifier
pins R and the exact record SHA256 in source. R is not named or self-hashed in
the record; the environment and command line cannot supply authority.

The first record binds the complete singleton correction: imported owner O,
source S, direct correction C whose only parent is O, reviewed donor D, every
mode/type/blob before and after, and the unchanged Voice013 UP prerequisite.
The original content base and runtime trigger are pinned too; a caller cannot
substitute a later ancestor row to hide retained content or runtime taint.
No Matchmaking migration is included. The independent donor review explicitly
covers the retained-counter/outbox DOWN guard; migration PostgreSQL proof is
reused only while those exact bytes and prerequisites remain unchanged.

The verifier must inspect the real collector map and real Git objects. It must
require exactly one prior row; exact O/S and complete correction delta; C in
the actual develop first-parent range; R after C and in the selected target;
matching donor entries; unchanged prerequisites at O/C/target; and final target
retention. Missing, renamed, extra, duplicate, reordered or partially specified
entries fail closed, as do mode/type changes or unrecorded absence. Record text,
same bytes from unrelated ancestry, and caller-supplied pins are insufficient.

Qualified replacement keeps one C-owned row with actual base O/head C, the
original earliest content base, and previous runtime trigger OR current trigger.
Guarded entries and final retention checks remain active. No `continue` skips
the downstream path/content oracle. Retained prior forbidden activation must
still fail. Ordinary independent overlapping sources, octopus collisions,
historical release wrappers/defaults, master push-before/two-parent checks and
event selection retain their previous behavior.

Verification uses actual Linux Git histories and production collector/event
routing, including the exact failed history and finite positive/negative cases
from SWARM c24. Helper-only success is insufficient. Broad current develop R22
scope failures remain failures pending a separately reviewed complete source
checkpoint or documented successor scope contract; this record is not one.
