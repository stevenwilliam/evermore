-- Extensions are left in place on the way down. Dropping postgis would cascade
-- into every geography column in every other schema sharing this database, and
-- migrations are forward-only in production anyway (CLAUDE.md §4).
SELECT 1;
