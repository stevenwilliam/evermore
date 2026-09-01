-- 0001 — extensions.
--
-- postgis     : geography(Point/Polygon,4326) for kitchen coverage. The
--               polygon-over-radius rule in 01-domain-model.md §5.3 needs real
--               geometry, not a lat/lng box.
-- btree_gist  : required by the EXCLUDE constraints on the four price tables,
--               which mix an equality on scope_key with an overlap on a
--               daterange (§3.5).
-- citext      : case-insensitive email, so two accounts cannot differ only by
--               capitalisation.
-- pgcrypto    : gen_random_uuid() for defaults; application IDs are UUIDv7
--               minted in Go, this is the safety net for rows the app does not
--               author.

CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS btree_gist;
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pgcrypto;
