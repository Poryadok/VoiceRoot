-- Story claims must check immutable/retired owner identity across File aliases.
CREATE INDEX file_references_story_owner_idx ON file_references(owner_id) WHERE owner_type=2;
