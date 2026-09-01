-- 0006 — orders, lines, payments, packages, credits, deliveries.
-- 01-domain-model.md §2.4, §3.6, §3.7, §3.9.

-- ---------------------------------------------------------------------------
-- Monthly order sequence. Allocating the number in the database is what makes
-- EVM-2609-0148 unique under concurrency; two checkouts in the same
-- millisecond cannot both take 0148.
-- ---------------------------------------------------------------------------
CREATE TABLE order_sequence (
    prefix     text NOT NULL,
    period     text NOT NULL,          -- YYMM
    last_value int  NOT NULL DEFAULT 0,
    PRIMARY KEY (prefix, period),

    CONSTRAINT order_sequence_value_positive CHECK (last_value >= 0)
);

CREATE OR REPLACE FUNCTION next_order_number(p_prefix text, p_period text)
RETURNS text LANGUAGE plpgsql AS $$
DECLARE
    v_next int;
BEGIN
    INSERT INTO order_sequence (prefix, period, last_value)
         VALUES (p_prefix, p_period, 1)
    ON CONFLICT (prefix, period)
      DO UPDATE SET last_value = order_sequence.last_value + 1
      RETURNING last_value INTO v_next;

    RETURN p_prefix || '-' || p_period || '-' || lpad(v_next::text, 4, '0');
END;
$$;

-- ---------------------------------------------------------------------------
-- Orders.
-- ---------------------------------------------------------------------------
CREATE TABLE customer_order (
    id                    uuid PRIMARY KEY,
    order_number          text NOT NULL,
    customer_id           uuid NOT NULL REFERENCES customer(id) ON DELETE RESTRICT,
    order_type            text NOT NULL,
    status                text NOT NULL DEFAULT 'DRAFT',

    -- Every amount is whole rupiah in BIGINT (CLAUDE.md §4).
    subtotal_idr          bigint NOT NULL DEFAULT 0,
    delivery_fee_idr      bigint NOT NULL DEFAULT 0,
    discount_idr          bigint NOT NULL DEFAULT 0,
    total_idr             bigint NOT NULL DEFAULT 0,

    -- The tax split, snapshotted. tax_idr is the SUM of the line taxes and is
    -- never re-derived from total_idr (§3.11).
    tax_base_idr          bigint NOT NULL DEFAULT 0,
    tax_idr               bigint NOT NULL DEFAULT 0,
    tax_rate_bps          int    NOT NULL DEFAULT 0,

    -- The payment matching suffix (D-16). payment_rounding_idr is the delta so
    -- reports reconcile, and it is excluded from the tax base.
    payment_amount_idr    bigint NOT NULL DEFAULT 0,
    payment_rounding_idr  bigint NOT NULL DEFAULT 0,
    payment_deadline_at   timestamptz,

    idempotency_key       text,
    price_resolution_trace jsonb NOT NULL DEFAULT '{}'::jsonb,
    notes                 text NOT NULL DEFAULT '',
    cancelled_reason      text,
    placed_at             timestamptz,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT customer_order_type_known  CHECK (order_type IN ('MEAL','PACKAGE')),
    CONSTRAINT customer_order_status_known CHECK (status IN
        ('DRAFT','AWAITING_PAYMENT','PAYMENT_SUBMITTED','PAID','COMPLETED',
         'CANCELLED','EXPIRED','REFUNDED')),
    CONSTRAINT customer_order_amounts_non_negative CHECK (
        subtotal_idr >= 0 AND delivery_fee_idr >= 0 AND discount_idr >= 0 AND
        total_idr >= 0 AND tax_base_idr >= 0 AND tax_idr >= 0 AND
        payment_amount_idr >= 0 AND payment_rounding_idr >= 0),
    CONSTRAINT customer_order_tax_rate_sane CHECK (tax_rate_bps BETWEEN 0 AND 10000),

    -- base + tax must equal the taxable total, always. This is the money
    -- invariant from §3.11 stated where the database can refuse a violation,
    -- rather than trusted to the application that computes it.
    CONSTRAINT customer_order_tax_reconciles
        CHECK (tax_base_idr + tax_idr = total_idr),

    -- The suffix is the only difference between what is owed and what is
    -- transferred, and it is under one thousand rupiah plus a carry.
    CONSTRAINT customer_order_payment_reconciles
        CHECK (payment_amount_idr = total_idr + payment_rounding_idr),

    -- A cancelled order must say why. Nothing automated cancels a booking
    -- (CLAUDE.md §7), so there is always a human with a reason.
    CONSTRAINT customer_order_cancelled_has_reason
        CHECK (status <> 'CANCELLED' OR length(btrim(coalesce(cancelled_reason,''))) > 0)
);

CREATE UNIQUE INDEX customer_order_number_uk ON customer_order (order_number);
CREATE UNIQUE INDEX customer_order_idempotency_uk
    ON customer_order (customer_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
CREATE INDEX customer_order_customer_ix ON customer_order (customer_id, created_at DESC);
CREATE INDEX customer_order_status_ix   ON customer_order (status, payment_deadline_at)
    WHERE status IN ('AWAITING_PAYMENT','PAYMENT_SUBMITTED');

-- One open payment suffix per bank account per day, so an incoming transfer
-- matches exactly one order. Enforced as a partial unique index rather than
-- checked in the application, because two checkouts can race.
CREATE TABLE payment_suffix_claim (
    id           uuid PRIMARY KEY,
    order_id     uuid NOT NULL REFERENCES customer_order(id) ON DELETE CASCADE,
    claim_date   date NOT NULL,
    suffix       int  NOT NULL,
    released_at  timestamptz,

    CONSTRAINT payment_suffix_range CHECK (suffix BETWEEN 0 AND 999)
);
CREATE UNIQUE INDEX payment_suffix_claim_uk
    ON payment_suffix_claim (claim_date, suffix)
    WHERE released_at IS NULL;

CREATE TABLE order_line (
    id                 uuid PRIMARY KEY,
    order_id           uuid NOT NULL REFERENCES customer_order(id) ON DELETE CASCADE,
    scheduled_meal_id  uuid REFERENCES scheduled_meal(id) ON DELETE RESTRICT,
    package_id         uuid REFERENCES package(id) ON DELETE RESTRICT,
    qty                int NOT NULL,

    unit_price_idr     bigint NOT NULL,
    normal_price_idr   bigint NOT NULL,
    line_total_idr     bigint NOT NULL,
    line_tax_base_idr  bigint NOT NULL,
    line_tax_idr       bigint NOT NULL,

    is_promo           boolean NOT NULL DEFAULT false,
    -- Snapshots, deliberately without a FK: a price row may be archived or a
    -- food edited, and the order must still say what it actually sold.
    price_row_id       uuid,
    price_table        text,
    meal_snapshot      jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at         timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT order_line_qty_positive CHECK (qty > 0),
    CONSTRAINT order_line_amounts_non_negative CHECK (
        unit_price_idr >= 0 AND normal_price_idr >= 0 AND line_total_idr >= 0 AND
        line_tax_base_idr >= 0 AND line_tax_idr >= 0),
    CONSTRAINT order_line_total_is_unit_times_qty
        CHECK (line_total_idr = unit_price_idr * qty),
    CONSTRAINT order_line_tax_reconciles
        CHECK (line_tax_base_idr + line_tax_idr = line_total_idr),
    CONSTRAINT order_line_price_table_known
        CHECK (price_table IS NULL OR price_table IN
               ('meal_normal','meal_promo','pkg_normal','pkg_promo')),
    -- A line is either a meal line or a package line, never both and never
    -- neither.
    CONSTRAINT order_line_exactly_one_subject
        CHECK ((scheduled_meal_id IS NOT NULL)::int + (package_id IS NOT NULL)::int = 1)
);
CREATE INDEX order_line_order_ix ON order_line (order_id);
CREATE INDEX order_line_meal_ix  ON order_line (scheduled_meal_id) WHERE scheduled_meal_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- Payments.
-- ---------------------------------------------------------------------------
CREATE TABLE payment (
    id                uuid PRIMARY KEY,
    order_id          uuid NOT NULL REFERENCES customer_order(id) ON DELETE RESTRICT,
    bank_account_id   uuid REFERENCES bank_account(id) ON DELETE RESTRICT,
    provider          text NOT NULL DEFAULT 'MANUAL_TRANSFER',
    status            text NOT NULL DEFAULT 'PENDING',
    expected_amount_idr bigint NOT NULL,
    declared_amount_idr bigint,
    sender_name       text,
    transferred_at    timestamptz,
    submitted_at      timestamptz,
    verified_at       timestamptz,
    verified_by       uuid REFERENCES app_user(id),
    rejected_reason   text,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT payment_status_known CHECK (status IN
        ('PENDING','SUBMITTED','VERIFIED','REJECTED','EXPIRED','REFUNDED')),
    CONSTRAINT payment_amount_positive CHECK (expected_amount_idr > 0),
    CONSTRAINT payment_rejected_has_reason
        CHECK (status <> 'REJECTED' OR length(btrim(coalesce(rejected_reason,''))) > 0),
    CONSTRAINT payment_verified_has_actor
        CHECK (status <> 'VERIFIED' OR (verified_by IS NOT NULL AND verified_at IS NOT NULL))
);
CREATE INDEX payment_order_ix  ON payment (order_id);
CREATE INDEX payment_queue_ix  ON payment (status, submitted_at) WHERE status = 'SUBMITTED';

CREATE TABLE payment_proof (
    id          uuid PRIMARY KEY,
    payment_id  uuid NOT NULL REFERENCES payment(id) ON DELETE CASCADE,
    object_key  text NOT NULL,
    mime_type   text NOT NULL,
    size_bytes  bigint NOT NULL,
    sha256      text NOT NULL,
    uploaded_at timestamptz NOT NULL DEFAULT now(),
    uploaded_by uuid REFERENCES app_user(id),

    -- The artifact says JPG or PNG, maximum 5 MB. The limit is enforced here
    -- as well as at the handler, because the handler can be bypassed.
    CONSTRAINT payment_proof_mime_allowed CHECK (mime_type IN ('image/jpeg','image/png')),
    CONSTRAINT payment_proof_size_limit   CHECK (size_bytes > 0 AND size_bytes <= 5242880)
);
CREATE INDEX payment_proof_payment_ix ON payment_proof (payment_id);

-- Append-only history of every payment state change.
CREATE TABLE payment_event (
    id          uuid PRIMARY KEY,
    payment_id  uuid NOT NULL REFERENCES payment(id) ON DELETE CASCADE,
    from_status text,
    to_status   text NOT NULL,
    actor_id    uuid REFERENCES app_user(id),
    reason      text,
    occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX payment_event_payment_ix ON payment_event (payment_id, occurred_at);
CREATE TRIGGER payment_event_append_only
    BEFORE UPDATE OR DELETE ON payment_event
    FOR EACH ROW EXECUTE FUNCTION refuse_mutation();

-- ---------------------------------------------------------------------------
-- Customer packages and the credit ledger.
-- ---------------------------------------------------------------------------
CREATE TABLE customer_package (
    id             uuid PRIMARY KEY,
    customer_id    uuid NOT NULL REFERENCES customer(id) ON DELETE RESTRICT,
    package_id     uuid NOT NULL REFERENCES package(id) ON DELETE RESTRICT,
    order_id       uuid REFERENCES customer_order(id) ON DELETE RESTRICT,
    package_number text NOT NULL,
    -- Snapshots: the catalogue may change, this purchase may not.
    meal_credits   int NOT NULL,
    validity_days  int NOT NULL,
    price_paid_idr bigint NOT NULL,
    status         text NOT NULL DEFAULT 'PENDING',
    activated_at   timestamptz,
    -- A DATE in Asia/Jakarta, not a timestamp: expiry is a business day.
    expires_at     date,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT customer_package_status_known CHECK (status IN
        ('PENDING','ACTIVE','EXHAUSTED','EXPIRED','CANCELLED')),
    CONSTRAINT customer_package_credits_positive CHECK (meal_credits > 0),
    CONSTRAINT customer_package_validity_positive CHECK (validity_days > 0),
    -- An active package has been activated and has an expiry. D-14.
    CONSTRAINT customer_package_active_is_dated
        CHECK (status = 'PENDING' OR status = 'CANCELLED'
               OR (activated_at IS NOT NULL AND expires_at IS NOT NULL))
);
CREATE UNIQUE INDEX customer_package_number_uk ON customer_package (package_number);
CREATE INDEX customer_package_customer_ix ON customer_package (customer_id, status);
CREATE INDEX customer_package_expiry_ix   ON customer_package (expires_at) WHERE status = 'ACTIVE';

-- The credit ledger is append-only and the balance is never stored: it is
-- SUM(qty). CLAUDE.md §4 requires the migration to spell that out, and the
-- trigger below is what makes it true rather than merely stated.
CREATE TABLE credit_ledger (
    id                  uuid PRIMARY KEY,
    customer_id         uuid NOT NULL REFERENCES customer(id) ON DELETE RESTRICT,
    customer_package_id uuid NOT NULL REFERENCES customer_package(id) ON DELETE RESTRICT,
    entry_type          text NOT NULL,
    qty                 int NOT NULL,
    reference_type      text NOT NULL DEFAULT '',
    reference_id        uuid,
    occurred_at         timestamptz NOT NULL DEFAULT now(),
    created_by          uuid REFERENCES app_user(id),
    note                text NOT NULL DEFAULT '',

    CONSTRAINT credit_ledger_entry_type_known CHECK (entry_type IN
        ('PURCHASE','REDEEM','REFUND','EXPIRE','ADJUSTMENT')),
    CONSTRAINT credit_ledger_qty_non_zero CHECK (qty <> 0),
    -- Sign discipline per entry type, so a REDEEM can never add credit.
    CONSTRAINT credit_ledger_sign_matches_type CHECK (
        (entry_type = 'PURCHASE'   AND qty > 0) OR
        (entry_type = 'REDEEM'     AND qty < 0) OR
        (entry_type = 'REFUND'     AND qty > 0) OR
        (entry_type = 'EXPIRE'     AND qty < 0) OR
        (entry_type = 'ADJUSTMENT')
    ),
    -- An adjustment must say why (§5.2).
    CONSTRAINT credit_ledger_adjustment_has_note
        CHECK (entry_type <> 'ADJUSTMENT' OR length(btrim(note)) > 0)
);
CREATE INDEX credit_ledger_package_ix  ON credit_ledger (customer_package_id, occurred_at);
CREATE INDEX credit_ledger_customer_ix ON credit_ledger (customer_id, occurred_at DESC);

CREATE TRIGGER credit_ledger_append_only
    BEFORE UPDATE OR DELETE ON credit_ledger
    FOR EACH ROW EXECUTE FUNCTION refuse_mutation();

-- ---------------------------------------------------------------------------
-- Deliveries.
-- ---------------------------------------------------------------------------
CREATE TABLE delivery (
    id                  uuid PRIMARY KEY,
    delivery_number     text NOT NULL,
    order_id            uuid REFERENCES customer_order(id) ON DELETE RESTRICT,
    customer_package_id uuid REFERENCES customer_package(id) ON DELETE RESTRICT,
    customer_id         uuid NOT NULL REFERENCES customer(id) ON DELETE RESTRICT,
    service_date        date NOT NULL,
    slot_id             uuid NOT NULL REFERENCES delivery_time_slot(id) ON DELETE RESTRICT,
    diet_type_id        uuid NOT NULL REFERENCES diet_type(id) ON DELETE RESTRICT,
    kitchen_id          uuid NOT NULL REFERENCES kitchen(id) ON DELETE RESTRICT,
    address_id          uuid REFERENCES customer_address(id) ON DELETE SET NULL,
    -- The address as it was at confirmation. A customer editing their address
    -- later must not silently redirect a delivery already in the kitchen queue.
    address_snapshot    jsonb NOT NULL,
    assigned_distance_m int,
    assignment_mode     text NOT NULL DEFAULT 'AUTO',
    assignment_reason   text NOT NULL DEFAULT '',
    assigned_at         timestamptz NOT NULL DEFAULT now(),
    status              text NOT NULL DEFAULT 'SCHEDULED',
    delivered_at        timestamptz,
    failure_reason      text,
    courier_note        text NOT NULL DEFAULT '',
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT delivery_status_known CHECK (status IN
        ('SCHEDULED','PREPARING','OUT_FOR_DELIVERY','DELIVERED','FAILED','SKIPPED','CANCELLED')),
    CONSTRAINT delivery_mode_known CHECK (assignment_mode IN ('AUTO','MANUAL')),
    -- A delivery comes from an order or from a package redemption, never both.
    CONSTRAINT delivery_exactly_one_source
        CHECK ((order_id IS NOT NULL)::int + (customer_package_id IS NOT NULL)::int = 1),
    CONSTRAINT delivery_failed_has_reason
        CHECK (status <> 'FAILED' OR length(btrim(coalesce(failure_reason,''))) > 0),
    CONSTRAINT delivery_delivered_is_timed
        CHECK (status <> 'DELIVERED' OR delivered_at IS NOT NULL)
);
CREATE UNIQUE INDEX delivery_number_uk ON delivery (delivery_number);
CREATE INDEX delivery_kitchen_day_ix ON delivery (kitchen_id, service_date, slot_id);
CREATE INDEX delivery_customer_ix    ON delivery (customer_id, service_date DESC);
CREATE INDEX delivery_status_ix      ON delivery (service_date, status);

CREATE TABLE delivery_line (
    id                uuid PRIMARY KEY,
    delivery_id       uuid NOT NULL REFERENCES delivery(id) ON DELETE CASCADE,
    scheduled_meal_id uuid NOT NULL REFERENCES scheduled_meal(id) ON DELETE RESTRICT,
    order_line_id     uuid REFERENCES order_line(id) ON DELETE SET NULL,
    qty               int NOT NULL DEFAULT 1,
    meal_snapshot     jsonb NOT NULL DEFAULT '{}'::jsonb,

    CONSTRAINT delivery_line_qty_positive CHECK (qty > 0)
);
CREATE INDEX delivery_line_delivery_ix ON delivery_line (delivery_id);

CREATE TRIGGER customer_order_touch    BEFORE UPDATE ON customer_order    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER payment_touch           BEFORE UPDATE ON payment           FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER customer_package_touch  BEFORE UPDATE ON customer_package  FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER delivery_touch          BEFORE UPDATE ON delivery          FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
