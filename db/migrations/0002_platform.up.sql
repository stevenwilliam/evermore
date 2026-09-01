-- 0002 — platform tables.
--
-- These are business-agnostic and reference nothing. They are omitted from the
-- ERDs in 01-domain-model.md §2 for that reason.

-- ---------------------------------------------------------------------------
-- sys_parameters — CLAUDE.md §7: anything that could change without a code
-- change is a row here, not a constant. Company phone, tax rate, cut-off time,
-- lead times, capacities, feature toggles.
-- ---------------------------------------------------------------------------
CREATE TABLE sys_parameters (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    key           text NOT NULL,
    value         text NOT NULL,
    value_type    text NOT NULL DEFAULT 'string',
    category      text NOT NULL DEFAULT 'general',
    label_id      text NOT NULL,
    label_en      text NOT NULL DEFAULT '',
    description   text NOT NULL DEFAULT '',
    -- A secret-flagged value is masked in the UI and in logs. The masking is
    -- the application's job; this column is what tells it to.
    is_secret     boolean NOT NULL DEFAULT false,
    is_active     boolean NOT NULL DEFAULT true,
    updated_by    uuid,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT sys_parameters_key_format
        CHECK (key ~ '^[a-z][a-z0-9_.]*$'),
    CONSTRAINT sys_parameters_value_type_known
        CHECK (value_type IN ('string','int','bool','decimal','json','time','date'))
);

CREATE UNIQUE INDEX sys_parameters_key_uk ON sys_parameters (key);
CREATE INDEX sys_parameters_category_ix ON sys_parameters (category) WHERE is_active;

-- ---------------------------------------------------------------------------
-- audit_log — append-only. CLAUDE.md §4: history tables take no UPDATE and no
-- DELETE, and the migration says so. The rule is enforced by a trigger below,
-- not left to convention.
-- ---------------------------------------------------------------------------
CREATE TABLE audit_log (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id      uuid,
    actor_email   text,
    actor_role    text,
    action        text NOT NULL,
    entity_type   text NOT NULL,
    entity_id     uuid,
    -- before/after are redacted by the application for secret-flagged fields
    -- before they ever reach this table.
    before_state  jsonb,
    after_state   jsonb,
    reason        text,
    ip_address    inet,
    user_agent    text,
    occurred_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX audit_log_entity_ix   ON audit_log (entity_type, entity_id, occurred_at DESC);
CREATE INDEX audit_log_actor_ix    ON audit_log (actor_id, occurred_at DESC);
CREATE INDEX audit_log_occurred_ix ON audit_log (occurred_at DESC);

-- ---------------------------------------------------------------------------
-- notification_log — every outbound message, append-only.
-- ---------------------------------------------------------------------------
CREATE TABLE notification_log (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    channel       text NOT NULL,
    recipient     text NOT NULL,
    template_key  text NOT NULL,
    locale        text NOT NULL DEFAULT 'id-ID',
    subject       text,
    body_preview  text,
    status        text NOT NULL DEFAULT 'QUEUED',
    provider_ref  text,
    error_detail  text,
    entity_type   text,
    entity_id     uuid,
    queued_at     timestamptz NOT NULL DEFAULT now(),
    sent_at       timestamptz,

    CONSTRAINT notification_log_channel_known
        CHECK (channel IN ('email','whatsapp','sms')),
    CONSTRAINT notification_log_status_known
        CHECK (status IN ('QUEUED','SENT','FAILED','SKIPPED'))
);

CREATE INDEX notification_log_entity_ix ON notification_log (entity_type, entity_id);
CREATE INDEX notification_log_status_ix ON notification_log (status, queued_at) WHERE status = 'QUEUED';

-- ---------------------------------------------------------------------------
-- idempotency_key — so a retried checkout does not place two orders.
-- ---------------------------------------------------------------------------
CREATE TABLE idempotency_key (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    key            text NOT NULL,
    scope          text NOT NULL,
    user_id        uuid,
    request_hash   text NOT NULL,
    response_status int,
    response_body  jsonb,
    state          text NOT NULL DEFAULT 'IN_FLIGHT',
    created_at     timestamptz NOT NULL DEFAULT now(),
    completed_at   timestamptz,
    expires_at     timestamptz NOT NULL,

    CONSTRAINT idempotency_state_known
        CHECK (state IN ('IN_FLIGHT','COMPLETED'))
);

CREATE UNIQUE INDEX idempotency_key_uk ON idempotency_key (scope, key);
CREATE INDEX idempotency_expiry_ix ON idempotency_key (expires_at);

-- ---------------------------------------------------------------------------
-- job — scheduled work: expire unpaid orders, publish tomorrow's menu, post
-- package expiries.
-- ---------------------------------------------------------------------------
CREATE TABLE job (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    job_type      text NOT NULL,
    payload       jsonb NOT NULL DEFAULT '{}'::jsonb,
    status        text NOT NULL DEFAULT 'PENDING',
    attempts      int NOT NULL DEFAULT 0,
    max_attempts  int NOT NULL DEFAULT 5,
    last_error    text,
    run_after     timestamptz NOT NULL DEFAULT now(),
    locked_at     timestamptz,
    locked_by     text,
    created_at    timestamptz NOT NULL DEFAULT now(),
    finished_at   timestamptz,

    CONSTRAINT job_status_known
        CHECK (status IN ('PENDING','RUNNING','DONE','FAILED','DEAD')),
    CONSTRAINT job_attempts_sane
        CHECK (attempts >= 0 AND attempts <= max_attempts)
);

CREATE INDEX job_queue_ix ON job (status, run_after) WHERE status = 'PENDING';

-- ---------------------------------------------------------------------------
-- Append-only enforcement.
--
-- CLAUDE.md §4 says history tables are append-only and the migration spells
-- that out. A comment is not enforcement, so this trigger is. It is attached
-- to audit_log and credit_ledger (the latter in 0006, once that table exists).
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION refuse_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION
        'table % is append-only: % is refused',
        TG_TABLE_NAME, TG_OP
        USING ERRCODE = 'restrict_violation';
END;
$$;

CREATE TRIGGER audit_log_append_only
    BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION refuse_mutation();

-- ---------------------------------------------------------------------------
-- updated_at maintenance, so no handler has to remember.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION touch_updated_at() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

CREATE TRIGGER sys_parameters_touch
    BEFORE UPDATE ON sys_parameters
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
