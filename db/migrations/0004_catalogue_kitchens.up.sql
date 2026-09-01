-- 0004 — catalogue, menu calendar, kitchens, geography, capacity.
-- 01-domain-model.md §2.2 and §2.3.

-- ---------------------------------------------------------------------------
-- Diet types and the food catalogue.
-- ---------------------------------------------------------------------------
CREATE TABLE diet_type (
    id             uuid PRIMARY KEY,
    name           text NOT NULL,
    slug           text NOT NULL,
    description    text NOT NULL DEFAULT '',
    hero_image_key text,
    sort_order     int NOT NULL DEFAULT 0,
    has_subtypes   boolean NOT NULL DEFAULT false,
    is_active      boolean NOT NULL DEFAULT true,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX diet_type_slug_uk ON diet_type (slug);

CREATE TABLE diet_subtype (
    id           uuid PRIMARY KEY,
    diet_type_id uuid NOT NULL REFERENCES diet_type(id) ON DELETE CASCADE,
    name         text NOT NULL,
    slug         text NOT NULL,
    is_active    boolean NOT NULL DEFAULT true
);
CREATE UNIQUE INDEX diet_subtype_slug_uk ON diet_subtype (diet_type_id, slug);

CREATE TABLE allergen (
    id      uuid PRIMARY KEY,
    name_id text NOT NULL,
    name_en text NOT NULL,
    slug    text NOT NULL
);
CREATE UNIQUE INDEX allergen_slug_uk ON allergen (slug);

CREATE TABLE food (
    id           uuid PRIMARY KEY,
    name         text NOT NULL,
    slug         text NOT NULL,
    description  text NOT NULL DEFAULT '',
    portion_size text NOT NULL DEFAULT '',
    is_active    boolean NOT NULL DEFAULT true,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT food_name_not_blank CHECK (length(btrim(name)) > 0)
);
CREATE UNIQUE INDEX food_slug_uk ON food (slug);
CREATE INDEX food_name_ix ON food (lower(name));

-- Nutrition is integers in base units so a meal's panel sums exactly
-- (01-domain-model.md §5.2b). Every quantity is non-negative.
CREATE TABLE food_nutrition (
    id                uuid PRIMARY KEY,
    food_id           uuid NOT NULL REFERENCES food(id) ON DELETE CASCADE,
    calories_kcal     int NOT NULL DEFAULT 0,
    protein_mg        int NOT NULL DEFAULT 0,
    fat_mg            int NOT NULL DEFAULT 0,
    saturated_fat_mg  int NOT NULL DEFAULT 0,
    carbohydrate_mg   int NOT NULL DEFAULT 0,
    sugar_mg          int NOT NULL DEFAULT 0,
    fibre_mg          int NOT NULL DEFAULT 0,
    sodium_mg         int NOT NULL DEFAULT 0,
    cholesterol_mg    int NOT NULL DEFAULT 0,
    extras            jsonb NOT NULL DEFAULT '{}'::jsonb,
    updated_at        timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT food_nutrition_non_negative CHECK (
        calories_kcal >= 0 AND protein_mg >= 0 AND fat_mg >= 0 AND
        saturated_fat_mg >= 0 AND carbohydrate_mg >= 0 AND sugar_mg >= 0 AND
        fibre_mg >= 0 AND sodium_mg >= 0 AND cholesterol_mg >= 0
    )
);
CREATE UNIQUE INDEX food_nutrition_food_uk ON food_nutrition (food_id);

CREATE TABLE food_allergen (
    food_id     uuid NOT NULL REFERENCES food(id) ON DELETE CASCADE,
    allergen_id uuid NOT NULL REFERENCES allergen(id) ON DELETE RESTRICT,
    PRIMARY KEY (food_id, allergen_id)
);

CREATE TABLE food_diet_type (
    food_id      uuid NOT NULL REFERENCES food(id) ON DELETE CASCADE,
    diet_type_id uuid NOT NULL REFERENCES diet_type(id) ON DELETE RESTRICT,
    PRIMARY KEY (food_id, diet_type_id)
);

CREATE TABLE food_photo (
    id         uuid PRIMARY KEY,
    food_id    uuid NOT NULL REFERENCES food(id) ON DELETE CASCADE,
    object_key text NOT NULL,
    alt_text   text NOT NULL DEFAULT '',
    sort_order int NOT NULL DEFAULT 0,
    is_primary boolean NOT NULL DEFAULT false
);
CREATE UNIQUE INDEX food_photo_one_primary_uk ON food_photo (food_id) WHERE is_primary;

-- ---------------------------------------------------------------------------
-- Delivery time slots — the 15-minute grid the artifact shows as 07.00, 11.30,
-- 12.00, 12.30, 17.30, 18.00, 18.30.
-- ---------------------------------------------------------------------------
CREATE TABLE delivery_time_slot (
    id         uuid PRIMARY KEY,
    slot_time  time NOT NULL,
    alias      text NOT NULL,
    meal_period text NOT NULL DEFAULT 'lunch',
    sort_order int NOT NULL DEFAULT 0,
    is_active  boolean NOT NULL DEFAULT true,

    -- The grid is 15 minutes. A slot at 11:37 would silently break every
    -- capacity join that assumes the grid, so the database refuses it.
    CONSTRAINT delivery_time_slot_on_grid
        CHECK (EXTRACT(MINUTE FROM slot_time)::int % 15 = 0
               AND EXTRACT(SECOND FROM slot_time) = 0),
    CONSTRAINT delivery_time_slot_period_known
        CHECK (meal_period IN ('breakfast','lunch','dinner'))
);
CREATE UNIQUE INDEX delivery_time_slot_time_uk ON delivery_time_slot (slot_time);

-- ---------------------------------------------------------------------------
-- The menu calendar. A scheduled_meal is what is sold; its items are the foods
-- that compose it (D-32/D-33). The calendar is global across kitchens (D-8),
-- so there is deliberately no kitchen_id here.
-- ---------------------------------------------------------------------------
CREATE TABLE scheduled_meal (
    id            uuid PRIMARY KEY,
    service_date  date NOT NULL,
    diet_type_id  uuid NOT NULL REFERENCES diet_type(id) ON DELETE RESTRICT,
    slot_id       uuid NOT NULL REFERENCES delivery_time_slot(id) ON DELETE RESTRICT,
    name          text NOT NULL,
    description   text NOT NULL DEFAULT '',
    qty_capacity  int,
    status        text NOT NULL DEFAULT 'DRAFT',
    published_at  timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT scheduled_meal_status_known CHECK (status IN ('DRAFT','PUBLISHED')),
    CONSTRAINT scheduled_meal_capacity_positive CHECK (qty_capacity IS NULL OR qty_capacity > 0),
    -- A PUBLISHED meal must carry the moment it was published; a DRAFT must
    -- not. Without this, "published_at IS NOT NULL" and "status = PUBLISHED"
    -- can disagree, and reports pick different ones.
    CONSTRAINT scheduled_meal_published_at_consistent
        CHECK ((status = 'PUBLISHED') = (published_at IS NOT NULL))
);

-- One meal per date + diet + slot. This is what makes the calendar a grid.
CREATE UNIQUE INDEX scheduled_meal_slot_uk
    ON scheduled_meal (service_date, diet_type_id, slot_id);
CREATE INDEX scheduled_meal_published_ix
    ON scheduled_meal (service_date, slot_id) WHERE status = 'PUBLISHED';

CREATE TABLE scheduled_meal_item (
    id               uuid PRIMARY KEY,
    scheduled_meal_id uuid NOT NULL REFERENCES scheduled_meal(id) ON DELETE CASCADE,
    food_id          uuid NOT NULL REFERENCES food(id) ON DELETE RESTRICT,
    item_role        text NOT NULL DEFAULT 'MAIN',
    portion_note     text NOT NULL DEFAULT '',
    sort_order       int NOT NULL DEFAULT 0,

    CONSTRAINT scheduled_meal_item_role_known
        CHECK (item_role IN ('MAIN','SIDE','DESSERT','DRINK'))
);
CREATE UNIQUE INDEX scheduled_meal_item_uk ON scheduled_meal_item (scheduled_meal_id, food_id);
CREATE INDEX scheduled_meal_item_meal_ix ON scheduled_meal_item (scheduled_meal_id);

-- ---------------------------------------------------------------------------
-- Kitchens and geography.
-- ---------------------------------------------------------------------------
CREATE TABLE kitchen (
    id                     uuid PRIMARY KEY,
    code                   text NOT NULL,
    name                   text NOT NULL,
    address_line           text NOT NULL DEFAULT '',
    latitude               numeric(10,7) NOT NULL,
    longitude              numeric(10,7) NOT NULL,
    geom                   geography(Point,4326)
                             GENERATED ALWAYS AS
                             (ST_SetSRID(ST_MakePoint(longitude::float8, latitude::float8), 4326)::geography)
                             STORED,
    service_radius_km      numeric(6,2) NOT NULL DEFAULT 5.0,
    -- A polygon overrides the radius entirely (§5.3). NULL means "use the
    -- radius"; a polygon means the radius is ignored for this kitchen.
    service_area           geography(Polygon,4326),
    phone                  text NOT NULL DEFAULT '',
    pic_name               text NOT NULL DEFAULT '',
    default_daily_capacity int NOT NULL DEFAULT 40,
    priority               int NOT NULL DEFAULT 100,
    is_active              boolean NOT NULL DEFAULT true,
    notes                  text NOT NULL DEFAULT '',
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT kitchen_lat_range    CHECK (latitude  BETWEEN -90  AND 90),
    CONSTRAINT kitchen_lng_range    CHECK (longitude BETWEEN -180 AND 180),
    CONSTRAINT kitchen_radius_positive CHECK (service_radius_km > 0),
    CONSTRAINT kitchen_capacity_positive CHECK (default_daily_capacity > 0)
);
CREATE UNIQUE INDEX kitchen_code_uk ON kitchen (code);
CREATE INDEX kitchen_geom_gix ON kitchen USING gist (geom);
CREATE INDEX kitchen_area_gix ON kitchen USING gist (service_area) WHERE service_area IS NOT NULL;

-- Deferred from 0003: staff are scoped to a kitchen (D-21).
ALTER TABLE staff_profile
    ADD CONSTRAINT staff_profile_kitchen_fk
    FOREIGN KEY (kitchen_id) REFERENCES kitchen(id) ON DELETE SET NULL;

CREATE TABLE kitchen_slot (
    kitchen_id uuid NOT NULL REFERENCES kitchen(id) ON DELETE CASCADE,
    slot_id    uuid NOT NULL REFERENCES delivery_time_slot(id) ON DELETE RESTRICT,
    is_active  boolean NOT NULL DEFAULT true,
    PRIMARY KEY (kitchen_id, slot_id)
);

CREATE TABLE kitchen_operating_day (
    kitchen_id  uuid NOT NULL REFERENCES kitchen(id) ON DELETE CASCADE,
    -- ISO-8601 weekday: 1 = Monday .. 7 = Sunday.
    weekday     int NOT NULL,
    is_open     boolean NOT NULL DEFAULT true,
    PRIMARY KEY (kitchen_id, weekday),

    CONSTRAINT kitchen_operating_day_weekday_range CHECK (weekday BETWEEN 1 AND 7)
);

-- ---------------------------------------------------------------------------
-- Capacity. This is the table the oversell test attacks.
--
-- CLAUDE.md §4: "If a counter must never exceed a maximum, a CHECK says so, so
-- the database refuses the bad write even under a race." That is exactly what
-- kitchen_capacity_not_oversold is. The application also takes
-- SELECT ... FOR UPDATE, but the CHECK is what makes the guarantee true rather
-- than merely likely.
-- ---------------------------------------------------------------------------
CREATE TABLE kitchen_capacity (
    id                uuid PRIMARY KEY,
    kitchen_id        uuid NOT NULL REFERENCES kitchen(id) ON DELETE CASCADE,
    service_date      date NOT NULL,
    slot_id           uuid NOT NULL REFERENCES delivery_time_slot(id) ON DELETE RESTRICT,
    max_portions      int NOT NULL,
    reserved_portions int NOT NULL DEFAULT 0,
    updated_at        timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT kitchen_capacity_max_positive     CHECK (max_portions >= 0),
    CONSTRAINT kitchen_capacity_reserved_positive CHECK (reserved_portions >= 0),
    CONSTRAINT kitchen_capacity_not_oversold      CHECK (reserved_portions <= max_portions)
);
CREATE UNIQUE INDEX kitchen_capacity_uk
    ON kitchen_capacity (kitchen_id, service_date, slot_id);
CREATE INDEX kitchen_capacity_date_ix ON kitchen_capacity (service_date, slot_id);

-- Every blocked checkout is logged, so "7 percobaan di luar jangkauan" on the
-- dashboard is a real count and not an estimate.
CREATE TABLE out_of_range_attempt (
    id              uuid PRIMARY KEY,
    customer_id     uuid REFERENCES customer(id) ON DELETE SET NULL,
    latitude        numeric(10,7) NOT NULL,
    longitude       numeric(10,7) NOT NULL,
    district        text NOT NULL DEFAULT '',
    slot_id         uuid REFERENCES delivery_time_slot(id) ON DELETE SET NULL,
    service_date    date,
    nearest_kitchen_id uuid REFERENCES kitchen(id) ON DELETE SET NULL,
    nearest_distance_m int,
    occurred_at     timestamptz NOT NULL DEFAULT now(),
    notify_requested boolean NOT NULL DEFAULT false,
    notify_email    citext
);
CREATE INDEX out_of_range_attempt_when_ix ON out_of_range_attempt (occurred_at DESC);

CREATE TRIGGER diet_type_touch        BEFORE UPDATE ON diet_type        FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER food_touch             BEFORE UPDATE ON food             FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER scheduled_meal_touch   BEFORE UPDATE ON scheduled_meal   FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER kitchen_touch          BEFORE UPDATE ON kitchen          FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER kitchen_capacity_touch BEFORE UPDATE ON kitchen_capacity FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
