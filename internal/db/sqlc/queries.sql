-- name: CreateApp :exec
INSERT INTO app (id, name, created_at)
VALUES (sqlc.arg(id), sqlc.arg(name), sqlc.arg(created_at));

-- name: GetApp :one
SELECT * FROM app WHERE id = ?;

-- name: UpdateAppKeyVersion :execrows
UPDATE app SET key_version = sqlc.arg(key_version), min_key_version = sqlc.arg(min_key_version) WHERE id = sqlc.arg(id);

-- name: UpsertAppCredential :exec
INSERT INTO app_credential (app_id, transport, enc_blob)
VALUES (sqlc.arg(app_id), sqlc.arg(transport), sqlc.arg(enc_blob))
ON CONFLICT (app_id, transport) DO UPDATE SET enc_blob = excluded.enc_blob;

-- name: DeleteAppCredential :exec
DELETE FROM app_credential WHERE app_id = sqlc.arg(app_id) AND transport = sqlc.arg(transport);

-- name: ListAppCredentials :many
SELECT transport, enc_blob FROM app_credential WHERE app_id = sqlc.arg(app_id);

-- name: CreateInstance :exec
INSERT INTO instance (id, app_id, label, enc_token_hash, created_at)
VALUES (sqlc.arg(id), sqlc.arg(app_id), sqlc.arg(label), sqlc.arg(enc_token_hash), sqlc.arg(created_at));

-- name: SetAppPublic :execrows
UPDATE app SET is_public = sqlc.arg(is_public) WHERE id = sqlc.arg(id);

-- name: SetAppTurnstile :execrows
UPDATE app SET turnstile_site_key = sqlc.arg(turnstile_site_key), enc_turnstile_secret = sqlc.arg(enc_turnstile_secret) WHERE id = sqlc.arg(id);

-- name: ClearAppTurnstile :execrows
UPDATE app SET turnstile_site_key = NULL, enc_turnstile_secret = NULL WHERE id = sqlc.arg(id);

-- name: UpdateApp :execrows
UPDATE app SET
  name = COALESCE(sqlc.narg(name), name),
  is_public = COALESCE(sqlc.narg(is_public), is_public)
WHERE id = sqlc.arg(id);

-- name: DeleteInstance :execrows
DELETE FROM instance WHERE id = sqlc.arg(id);

-- name: UpdateInstance :execrows
UPDATE instance SET
  label = COALESCE(sqlc.narg(label), label)
WHERE id = sqlc.arg(id);

-- name: UpdateInstanceTokenHash :execrows
UPDATE instance SET enc_token_hash = sqlc.arg(enc_token_hash) WHERE id = sqlc.arg(id);

-- name: SetAppToken :execrows
UPDATE app SET enc_token_hash = sqlc.arg(enc_token_hash) WHERE id = sqlc.arg(id);

-- name: GetInstance :one
SELECT * FROM instance WHERE id = ?;

-- name: CreateAppUsagePlan :exec
INSERT INTO app_usage_plan (id, app_id, name, push_per_min, push_daily_quota, register_per_min, register_burst, register_ip_per_min, register_ip_burst, subscribe_per_min, subscribe_burst, created_at)
VALUES (sqlc.arg(id), sqlc.narg(app_id), sqlc.arg(name), sqlc.arg(push_per_min), sqlc.arg(push_daily_quota), sqlc.arg(register_per_min), sqlc.arg(register_burst), sqlc.arg(register_ip_per_min), sqlc.arg(register_ip_burst), sqlc.arg(subscribe_per_min), sqlc.arg(subscribe_burst), sqlc.arg(created_at));

-- name: GetAppUsagePlan :one
SELECT * FROM app_usage_plan WHERE id = ?;

-- name: GetDefaultAppUsagePlan :one
SELECT * FROM app_usage_plan WHERE is_default = TRUE;

-- name: CreateDefaultAppUsagePlan :exec
INSERT INTO app_usage_plan (id, name, is_default, push_per_min, push_daily_quota, register_per_min, register_burst, register_ip_per_min, register_ip_burst, subscribe_per_min, subscribe_burst, created_at)
VALUES (sqlc.arg(id), sqlc.arg(name), TRUE, sqlc.arg(push_per_min), sqlc.arg(push_daily_quota), sqlc.arg(register_per_min), sqlc.arg(register_burst), sqlc.arg(register_ip_per_min), sqlc.arg(register_ip_burst), sqlc.arg(subscribe_per_min), sqlc.arg(subscribe_burst), sqlc.arg(created_at));

-- name: ListAppUsagePlans :many
SELECT * FROM app_usage_plan ORDER BY created_at;

-- name: UpdateAppUsagePlan :execrows
UPDATE app_usage_plan SET
  name = COALESCE(sqlc.narg(name), name),
  push_per_min = COALESCE(sqlc.narg(push_per_min), push_per_min),
  push_daily_quota = COALESCE(sqlc.narg(push_daily_quota), push_daily_quota),
  register_per_min = COALESCE(sqlc.narg(register_per_min), register_per_min),
  register_burst = COALESCE(sqlc.narg(register_burst), register_burst),
  register_ip_per_min = COALESCE(sqlc.narg(register_ip_per_min), register_ip_per_min),
  register_ip_burst = COALESCE(sqlc.narg(register_ip_burst), register_ip_burst),
  subscribe_per_min = COALESCE(sqlc.narg(subscribe_per_min), subscribe_per_min),
  subscribe_burst = COALESCE(sqlc.narg(subscribe_burst), subscribe_burst)
WHERE id = sqlc.arg(id);

-- name: DeleteAppUsagePlan :execrows
DELETE FROM app_usage_plan WHERE id = sqlc.arg(id);

-- name: AssignAppUsagePlan :execrows
UPDATE app SET usage_plan_id = sqlc.narg(usage_plan_id) WHERE id = sqlc.arg(id);

-- name: CreateInstanceUsagePlan :exec
INSERT INTO instance_usage_plan (id, app_id, instance_id, name, push_per_min, push_burst, push_daily_quota, created_at)
VALUES (sqlc.arg(id), sqlc.arg(app_id), sqlc.narg(instance_id), sqlc.arg(name), sqlc.arg(push_per_min), sqlc.arg(push_burst), sqlc.arg(push_daily_quota), sqlc.arg(created_at));

-- name: CreateDefaultInstanceUsagePlan :exec
INSERT INTO instance_usage_plan (id, app_id, name, is_default, push_per_min, push_burst, push_daily_quota, created_at)
VALUES (sqlc.arg(id), sqlc.arg(app_id), sqlc.arg(name), TRUE, sqlc.arg(push_per_min), sqlc.arg(push_burst), sqlc.arg(push_daily_quota), sqlc.arg(created_at));

-- name: GetInstanceUsagePlan :one
SELECT * FROM instance_usage_plan WHERE id = ?;

-- name: GetDefaultInstanceUsagePlan :one
SELECT * FROM instance_usage_plan WHERE app_id = sqlc.arg(app_id) AND is_default = TRUE;

-- name: ListInstanceUsagePlans :many
SELECT * FROM instance_usage_plan WHERE app_id = sqlc.arg(app_id) ORDER BY created_at;

-- name: UpdateInstanceUsagePlan :execrows
UPDATE instance_usage_plan SET
  name = COALESCE(sqlc.narg(name), name),
  push_per_min = COALESCE(sqlc.narg(push_per_min), push_per_min),
  push_burst = COALESCE(sqlc.narg(push_burst), push_burst),
  push_daily_quota = COALESCE(sqlc.narg(push_daily_quota), push_daily_quota)
WHERE id = sqlc.arg(id);

-- name: DeleteInstanceUsagePlan :execrows
DELETE FROM instance_usage_plan WHERE id = sqlc.arg(id) AND app_id = sqlc.arg(app_id);

-- name: AssignInstanceUsagePlan :execrows
UPDATE instance SET usage_plan_id = sqlc.narg(usage_plan_id) WHERE id = sqlc.arg(id);

-- name: ListApps :many
SELECT * FROM app ORDER BY created_at;

-- name: ListInstancesByApp :many
SELECT * FROM instance WHERE app_id = sqlc.arg(app_id) ORDER BY created_at;

-- name: UpsertUsageCounter :exec
INSERT INTO usage_counter (scope, date, count) VALUES (sqlc.arg(scope), sqlc.arg(date), sqlc.arg(count))
ON CONFLICT(scope, date) DO UPDATE SET count = excluded.count;

-- name: ListUsageCountersForDay :many
SELECT scope, count FROM usage_counter WHERE date = sqlc.arg(date);

-- name: DeleteUsageCountersBefore :exec
DELETE FROM usage_counter WHERE date < sqlc.arg(date);

-- name: CountApps :one
SELECT COUNT(*) FROM app;

-- name: CountInstances :one
SELECT COUNT(*) FROM instance;

-- name: GetServerSetting :one
SELECT * FROM settings WHERE id = 1;

-- name: UpdateServerSetting :exec
UPDATE settings SET
  retry_max_attempts = COALESCE(sqlc.narg(retry_max_attempts), retry_max_attempts),
  retry_base_backoff = COALESCE(sqlc.narg(retry_base_backoff), retry_base_backoff),
  retry_max_backoff = COALESCE(sqlc.narg(retry_max_backoff), retry_max_backoff),
  auth_fail_ip_per_min = COALESCE(sqlc.narg(auth_fail_ip_per_min), auth_fail_ip_per_min),
  auth_fail_ip_burst = COALESCE(sqlc.narg(auth_fail_ip_burst), auth_fail_ip_burst)
WHERE id = 1;
