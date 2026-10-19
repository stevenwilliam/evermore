-- D74 — what a package includes beyond its credits.
--
-- The packages page lists each tier's perks with a tick or a cross, so the
-- perks have to be DATA. Putting them in the template would mean the marketing
-- copy and the thing a customer actually buys could drift apart silently, and
-- the back office could never change a tier without a deploy.
--
-- Defaults are chosen so existing rows stay truthful: every current package
-- already includes delivery and allergen customisation, and none of them
-- included a dietician or daily fruit.
ALTER TABLE package
    ADD COLUMN free_delivery        boolean NOT NULL DEFAULT true,
    ADD COLUMN free_allergen_custom boolean NOT NULL DEFAULT true,
    ADD COLUMN dietician_sessions   int     NOT NULL DEFAULT 0,
    ADD COLUMN daily_fruit          boolean NOT NULL DEFAULT false;

-- A negative number of consultations is not a thing; the database says so
-- rather than trusting every writer to remember.
ALTER TABLE package
    ADD CONSTRAINT package_dietician_sessions_nonneg CHECK (dietician_sessions >= 0);
