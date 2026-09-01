DROP TRIGGER IF EXISTS sys_parameters_touch ON sys_parameters;
DROP TRIGGER IF EXISTS audit_log_append_only ON audit_log;
DROP TABLE IF EXISTS job, idempotency_key, notification_log, audit_log, sys_parameters CASCADE;
DROP FUNCTION IF EXISTS touch_updated_at();
DROP FUNCTION IF EXISTS refuse_mutation();
