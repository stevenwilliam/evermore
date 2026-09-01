-- 0003 — identity, RBAC, customers, organisations, addresses.
-- 01-domain-model.md §2.1.

-- ---------------------------------------------------------------------------
-- Users and RBAC. Deny-by-default: a user has no permission until a role
-- grants it, and every handler declares the permission it needs.
-- ---------------------------------------------------------------------------
CREATE TABLE app_user (
    id                 uuid PRIMARY KEY,
    email              citext NOT NULL,
    password_hash      text NOT NULL,
    phone              text,
    is_active          boolean NOT NULL DEFAULT true,
    email_verified_at  timestamptz,
    last_login_at      timestamptz,
    failed_login_count int NOT NULL DEFAULT 0,
    locked_until       timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),

    -- argon2id encoded hashes start with this prefix. The CHECK is cheap and
    -- makes it impossible to land a bcrypt or a plaintext value here by
    -- accident during a migration or a fixture load.
    CONSTRAINT app_user_password_is_argon2id
        CHECK (password_hash LIKE '$argon2id$%'),
    CONSTRAINT app_user_email_shape
        CHECK (email ~ '^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$'),
    CONSTRAINT app_user_failed_login_sane
        CHECK (failed_login_count >= 0)
);
CREATE UNIQUE INDEX app_user_email_uk ON app_user (email);

CREATE TABLE role (
    id          uuid PRIMARY KEY,
    name        text NOT NULL,
    slug        text NOT NULL,
    description text NOT NULL DEFAULT '',
    is_staff    boolean NOT NULL DEFAULT true,
    is_system   boolean NOT NULL DEFAULT false,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX role_slug_uk ON role (slug);

CREATE TABLE permission (
    id          uuid PRIMARY KEY,
    code        text NOT NULL,
    description text NOT NULL DEFAULT '',
    category    text NOT NULL DEFAULT 'general',

    CONSTRAINT permission_code_format CHECK (code ~ '^[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*$')
);
CREATE UNIQUE INDEX permission_code_uk ON permission (code);

CREATE TABLE role_permission (
    role_id       uuid NOT NULL REFERENCES role(id) ON DELETE CASCADE,
    permission_id uuid NOT NULL REFERENCES permission(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE user_role (
    user_id    uuid NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    role_id    uuid NOT NULL REFERENCES role(id) ON DELETE RESTRICT,
    granted_at timestamptz NOT NULL DEFAULT now(),
    granted_by uuid REFERENCES app_user(id),
    PRIMARY KEY (user_id, role_id)
);

-- Refresh tokens are stored hashed and are revocable; jti denylist on logout.
CREATE TABLE refresh_token (
    id           uuid PRIMARY KEY,
    user_id      uuid NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    token_hash   text NOT NULL,
    jti          uuid NOT NULL,
    issued_at    timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL,
    revoked_at   timestamptz,
    revoked_reason text,
    replaced_by  uuid REFERENCES refresh_token(id),
    user_agent   text,
    ip_address   inet,

    CONSTRAINT refresh_token_expires_after_issue CHECK (expires_at > issued_at)
);
CREATE UNIQUE INDEX refresh_token_hash_uk ON refresh_token (token_hash);
CREATE UNIQUE INDEX refresh_token_jti_uk  ON refresh_token (jti);
CREATE INDEX refresh_token_user_live_ix ON refresh_token (user_id) WHERE revoked_at IS NULL;

-- TOTP is mandatory for staff (D-21 scoping lives on staff_profile).
CREATE TABLE user_totp (
    id            uuid PRIMARY KEY,
    user_id       uuid NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    secret_cipher bytea NOT NULL,
    confirmed_at  timestamptz,
    last_used_step bigint,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX user_totp_user_uk ON user_totp (user_id);

CREATE TABLE totp_recovery_code (
    id         uuid PRIMARY KEY,
    user_id    uuid NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    code_hash  text NOT NULL,
    used_at    timestamptz
);
CREATE INDEX totp_recovery_user_ix ON totp_recovery_code (user_id) WHERE used_at IS NULL;

-- ---------------------------------------------------------------------------
-- Customers and organisations.
-- ---------------------------------------------------------------------------
CREATE TABLE customer_type (
    id           uuid PRIMARY KEY,
    name         text NOT NULL,
    slug         text NOT NULL,
    is_corporate boolean NOT NULL DEFAULT false,
    is_active    boolean NOT NULL DEFAULT true,
    sort_order   int NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX customer_type_slug_uk ON customer_type (slug);

CREATE TABLE organisation (
    id                 uuid PRIMARY KEY,
    customer_type_id   uuid REFERENCES customer_type(id),
    name               text NOT NULL,
    pic_name           text NOT NULL DEFAULT '',
    billing_email      citext,
    phone              text,
    po_number          text,
    npwp               text,
    is_invoice_billing boolean NOT NULL DEFAULT false,
    is_active          boolean NOT NULL DEFAULT true,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX organisation_name_ix ON organisation (lower(name));

CREATE TABLE customer (
    id               uuid PRIMARY KEY,
    user_id          uuid NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    customer_type_id uuid NOT NULL REFERENCES customer_type(id),
    organisation_id  uuid REFERENCES organisation(id),
    full_name        text NOT NULL,
    birth_date       date,
    gender           text,
    allergen_profile jsonb NOT NULL DEFAULT '[]'::jsonb,
    preferred_locale text NOT NULL DEFAULT 'id-ID',
    notify_channels  text NOT NULL DEFAULT 'email',
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT customer_gender_known
        CHECK (gender IS NULL OR gender IN ('male','female','other','undisclosed')),
    CONSTRAINT customer_locale_known
        CHECK (preferred_locale IN ('id-ID','en')),
    CONSTRAINT customer_full_name_not_blank
        CHECK (length(btrim(full_name)) > 0)
);
CREATE UNIQUE INDEX customer_user_uk ON customer (user_id);
CREATE INDEX customer_org_ix  ON customer (organisation_id) WHERE organisation_id IS NOT NULL;
CREATE INDEX customer_name_ix ON customer (lower(full_name));

-- Staff are scoped to a kitchen (D-21). The FK to kitchen is added in 0004,
-- once that table exists.
CREATE TABLE staff_profile (
    id         uuid PRIMARY KEY,
    user_id    uuid NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    kitchen_id uuid,
    full_name  text NOT NULL,
    job_title  text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX staff_profile_user_uk ON staff_profile (user_id);

-- ---------------------------------------------------------------------------
-- Addresses. latitude/longitude are NOT NULL because routing cannot work
-- without them, and geom is generated so it can never disagree with them.
-- ---------------------------------------------------------------------------
CREATE TABLE customer_address (
    id                 uuid PRIMARY KEY,
    customer_id        uuid NOT NULL REFERENCES customer(id) ON DELETE CASCADE,
    label              text NOT NULL DEFAULT 'Rumah',
    recipient_name     text NOT NULL,
    recipient_phone    text NOT NULL,
    address_line       text NOT NULL,
    district           text NOT NULL DEFAULT '',
    city               text NOT NULL DEFAULT '',
    province           text NOT NULL DEFAULT '',
    postal_code        text NOT NULL DEFAULT '',
    latitude           numeric(10,7) NOT NULL,
    longitude          numeric(10,7) NOT NULL,
    geom               geography(Point,4326)
                         GENERATED ALWAYS AS
                         (ST_SetSRID(ST_MakePoint(longitude::float8, latitude::float8), 4326)::geography)
                         STORED,
    google_place_id    text,
    formatted_address  text,
    driver_note        text NOT NULL DEFAULT '',
    is_default         boolean NOT NULL DEFAULT false,
    is_active          boolean NOT NULL DEFAULT true,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT customer_address_lat_range CHECK (latitude  BETWEEN -90  AND 90),
    CONSTRAINT customer_address_lng_range CHECK (longitude BETWEEN -180 AND 180),
    CONSTRAINT customer_address_recipient_not_blank
        CHECK (length(btrim(recipient_name)) > 0 AND length(btrim(recipient_phone)) > 0)
);

CREATE INDEX customer_address_customer_ix ON customer_address (customer_id) WHERE is_active;
CREATE INDEX customer_address_geom_gix    ON customer_address USING gist (geom);

-- Exactly one default address per customer. A partial unique index is what
-- makes "one per customer" the database's problem rather than a race the
-- application has to win.
CREATE UNIQUE INDEX customer_address_one_default_uk
    ON customer_address (customer_id)
    WHERE is_default AND is_active;

CREATE TRIGGER app_user_touch         BEFORE UPDATE ON app_user         FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER customer_touch         BEFORE UPDATE ON customer         FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER organisation_touch     BEFORE UPDATE ON organisation     FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER staff_profile_touch    BEFORE UPDATE ON staff_profile    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER customer_address_touch BEFORE UPDATE ON customer_address FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
