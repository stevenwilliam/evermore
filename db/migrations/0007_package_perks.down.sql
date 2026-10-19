ALTER TABLE package DROP CONSTRAINT IF EXISTS package_dietician_sessions_nonneg;
ALTER TABLE package
    DROP COLUMN IF EXISTS daily_fruit,
    DROP COLUMN IF EXISTS dietician_sessions,
    DROP COLUMN IF EXISTS free_allergen_custom,
    DROP COLUMN IF EXISTS free_delivery;
