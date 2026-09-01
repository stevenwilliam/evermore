-- 0005 — the four price tables, tiers, packages and bank accounts.
-- 01-domain-model.md §3.5.
--
-- Every price in all four tables is TAX-INCLUSIVE. The base/tax split is
-- computed at order time and stored on the order, never here: a rate change
-- from 11% to 12% would otherwise have to rewrite every row, and historical
-- rows would carry a rate they were never sold under (§3.11).

CREATE TABLE meal_price_tier (
    id         uuid PRIMARY KEY,
    name       text NOT NULL,
    min_qty    int NOT NULL,
    max_qty    int,                    -- NULL = unbounded
    sort_order int NOT NULL DEFAULT 0,
    is_active  boolean NOT NULL DEFAULT true,

    CONSTRAINT meal_price_tier_min_positive CHECK (min_qty >= 1),
    CONSTRAINT meal_price_tier_range_sane   CHECK (max_qty IS NULL OR max_qty >= min_qty)
);
-- Tier bands may not overlap. Gaps are checked by the application on save
-- (pricing.ValidateTiers) because a gap is an absence and an EXCLUDE
-- constraint can only see the rows that are present.
-- Half-open [) throughout. An inclusive '[]' upper bound would be normalised
-- by adding 1, and COALESCE(max_qty, 2147483647) + 1 overflows int4 — caught
-- by TestScopeKeyIsGeneratedAndCannotDrift before this shipped anywhere.
ALTER TABLE meal_price_tier ADD CONSTRAINT meal_price_tier_no_overlap
    EXCLUDE USING gist (
        int4range(min_qty, COALESCE(max_qty + 1, 2147483647), '[)') WITH &&
    ) WHERE (is_active);

CREATE TABLE package (
    id            uuid PRIMARY KEY,
    name          text NOT NULL,
    slug          text NOT NULL,
    description   text NOT NULL DEFAULT '',
    meal_credits  int NOT NULL,
    validity_days int NOT NULL,
    sort_order    int NOT NULL DEFAULT 0,
    is_featured   boolean NOT NULL DEFAULT false,
    is_active     boolean NOT NULL DEFAULT true,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT package_credits_positive CHECK (meal_credits > 0),
    CONSTRAINT package_validity_positive CHECK (validity_days > 0)
);
CREATE UNIQUE INDEX package_slug_uk ON package (slug);

-- D-12: a package may be restricted to particular diet types. No rows means
-- no restriction.
CREATE TABLE package_diet_type (
    package_id   uuid NOT NULL REFERENCES package(id) ON DELETE CASCADE,
    diet_type_id uuid NOT NULL REFERENCES diet_type(id) ON DELETE RESTRICT,
    PRIMARY KEY (package_id, diet_type_id)
);

-- ---------------------------------------------------------------------------
-- The four price tables.
--
-- scope_key is GENERATED from customer_type_id so it cannot drift. It exists
-- because an exclusion constraint cannot compare a nullable customer_type_id
-- with '=': NULL is not equal to NULL, so two DEFAULT rows would both be
-- accepted. That is why the brief specified it, and it is load-bearing (§3.5).
-- ---------------------------------------------------------------------------

CREATE TABLE meal_price_normal (
    id               uuid PRIMARY KEY,
    customer_type_id uuid REFERENCES customer_type(id) ON DELETE RESTRICT,
    scope_key        text GENERATED ALWAYS AS
                       (CASE WHEN customer_type_id IS NULL
                             THEN 'DEFAULT'
                             ELSE 'CT:' || customer_type_id::text END) STORED,
    diet_type_id     uuid NOT NULL REFERENCES diet_type(id) ON DELETE RESTRICT,
    tier_id          uuid NOT NULL REFERENCES meal_price_tier(id) ON DELETE RESTRICT,
    unit_price_idr   bigint NOT NULL,
    validity         daterange NOT NULL,
    is_active        boolean NOT NULL DEFAULT true,
    created_at       timestamptz NOT NULL DEFAULT now(),
    created_by       uuid REFERENCES app_user(id),

    CONSTRAINT meal_price_normal_price_positive CHECK (unit_price_idr > 0),
    CONSTRAINT meal_price_normal_validity_lower CHECK (lower(validity) IS NOT NULL),
    CONSTRAINT meal_price_normal_validity_bounds CHECK (lower_inc(validity) AND NOT upper_inc(validity))
);
ALTER TABLE meal_price_normal ADD CONSTRAINT meal_price_normal_no_overlap
    EXCLUDE USING gist (
        scope_key WITH =, diet_type_id WITH =, tier_id WITH =, validity WITH &&
    ) WHERE (is_active);
CREATE INDEX meal_price_normal_lookup_ix ON meal_price_normal (scope_key, diet_type_id, tier_id) WHERE is_active;

CREATE TABLE meal_price_promo (
    id               uuid PRIMARY KEY,
    customer_type_id uuid REFERENCES customer_type(id) ON DELETE RESTRICT,
    scope_key        text GENERATED ALWAYS AS
                       (CASE WHEN customer_type_id IS NULL
                             THEN 'DEFAULT'
                             ELSE 'CT:' || customer_type_id::text END) STORED,
    diet_type_id     uuid NOT NULL REFERENCES diet_type(id) ON DELETE RESTRICT,
    tier_id          uuid NOT NULL REFERENCES meal_price_tier(id) ON DELETE RESTRICT,
    unit_price_idr   bigint NOT NULL,
    validity         daterange NOT NULL,
    promo_label      text NOT NULL DEFAULT '',
    is_active        boolean NOT NULL DEFAULT true,
    created_at       timestamptz NOT NULL DEFAULT now(),
    created_by       uuid REFERENCES app_user(id),

    CONSTRAINT meal_price_promo_price_positive CHECK (unit_price_idr > 0),
    CONSTRAINT meal_price_promo_validity_lower CHECK (lower(validity) IS NOT NULL),
    CONSTRAINT meal_price_promo_validity_bounds CHECK (lower_inc(validity) AND NOT upper_inc(validity))
);
ALTER TABLE meal_price_promo ADD CONSTRAINT meal_price_promo_no_overlap
    EXCLUDE USING gist (
        scope_key WITH =, diet_type_id WITH =, tier_id WITH =, validity WITH &&
    ) WHERE (is_active);
CREATE INDEX meal_price_promo_lookup_ix ON meal_price_promo (scope_key, diet_type_id, tier_id) WHERE is_active;

CREATE TABLE package_price_normal (
    id               uuid PRIMARY KEY,
    customer_type_id uuid REFERENCES customer_type(id) ON DELETE RESTRICT,
    scope_key        text GENERATED ALWAYS AS
                       (CASE WHEN customer_type_id IS NULL
                             THEN 'DEFAULT'
                             ELSE 'CT:' || customer_type_id::text END) STORED,
    package_id       uuid NOT NULL REFERENCES package(id) ON DELETE RESTRICT,
    total_price_idr  bigint NOT NULL,
    validity         daterange NOT NULL,
    is_active        boolean NOT NULL DEFAULT true,
    created_at       timestamptz NOT NULL DEFAULT now(),
    created_by       uuid REFERENCES app_user(id),

    CONSTRAINT package_price_normal_price_positive CHECK (total_price_idr > 0),
    CONSTRAINT package_price_normal_validity_bounds CHECK (lower_inc(validity) AND NOT upper_inc(validity))
);
ALTER TABLE package_price_normal ADD CONSTRAINT package_price_normal_no_overlap
    EXCLUDE USING gist (
        scope_key WITH =, package_id WITH =, validity WITH &&
    ) WHERE (is_active);

CREATE TABLE package_price_promo (
    id               uuid PRIMARY KEY,
    customer_type_id uuid REFERENCES customer_type(id) ON DELETE RESTRICT,
    scope_key        text GENERATED ALWAYS AS
                       (CASE WHEN customer_type_id IS NULL
                             THEN 'DEFAULT'
                             ELSE 'CT:' || customer_type_id::text END) STORED,
    package_id       uuid NOT NULL REFERENCES package(id) ON DELETE RESTRICT,
    total_price_idr  bigint NOT NULL,
    validity         daterange NOT NULL,
    promo_label      text NOT NULL DEFAULT '',
    is_active        boolean NOT NULL DEFAULT true,
    created_at       timestamptz NOT NULL DEFAULT now(),
    created_by       uuid REFERENCES app_user(id),

    CONSTRAINT package_price_promo_price_positive CHECK (total_price_idr > 0),
    CONSTRAINT package_price_promo_validity_bounds CHECK (lower_inc(validity) AND NOT upper_inc(validity))
);
ALTER TABLE package_price_promo ADD CONSTRAINT package_price_promo_no_overlap
    EXCLUDE USING gist (
        scope_key WITH =, package_id WITH =, validity WITH &&
    ) WHERE (is_active);

-- ---------------------------------------------------------------------------
-- Bank accounts for manual transfer. The seeded row is a dummy (D14) and is
-- replaceable in the back office.
-- ---------------------------------------------------------------------------
CREATE TABLE bank_account (
    id             uuid PRIMARY KEY,
    bank_name      text NOT NULL,
    account_number text NOT NULL,
    account_holder text NOT NULL,
    branch         text NOT NULL DEFAULT '',
    is_active      boolean NOT NULL DEFAULT true,
    sort_order     int NOT NULL DEFAULT 0,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX bank_account_uk ON bank_account (bank_name, account_number);

-- ---------------------------------------------------------------------------
-- Delivery fee bands. D14: delivery is free at every distance and every order
-- value, but the band engine is still evaluated on every order so that
-- charging later is one settings edit rather than a code change.
-- ---------------------------------------------------------------------------
CREATE TABLE delivery_fee_band (
    id            uuid PRIMARY KEY,
    min_distance_m int NOT NULL DEFAULT 0,
    max_distance_m int,
    fee_idr       bigint NOT NULL DEFAULT 0,
    is_active     boolean NOT NULL DEFAULT true,

    CONSTRAINT delivery_fee_band_fee_non_negative CHECK (fee_idr >= 0),
    CONSTRAINT delivery_fee_band_range_sane
        CHECK (max_distance_m IS NULL OR max_distance_m > min_distance_m)
);
ALTER TABLE delivery_fee_band ADD CONSTRAINT delivery_fee_band_no_overlap
    EXCLUDE USING gist (
        int4range(min_distance_m, COALESCE(max_distance_m, 2147483647), '[)') WITH &&
    ) WHERE (is_active);

CREATE TRIGGER package_touch      BEFORE UPDATE ON package      FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER bank_account_touch BEFORE UPDATE ON bank_account FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
