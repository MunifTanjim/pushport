CREATE TABLE IF NOT EXISTS app (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	enc_token_hash BLOB,
	is_public BOOLEAN NOT NULL DEFAULT FALSE,
	key_version INTEGER NOT NULL DEFAULT 1,
	min_key_version INTEGER NOT NULL DEFAULT 1,
	turnstile_site_key TEXT,
	enc_turnstile_secret BLOB,
	created_at INTEGER NOT NULL,
	-- NO ACTION: refuse to delete an app_usage_plan while an app is assigned to it
	-- (the app must be reassigned first). It still tolerates the app-delete cascade.
	usage_plan_id TEXT REFERENCES app_usage_plan(id) ON DELETE NO ACTION
);

CREATE TABLE IF NOT EXISTS instance (
	id TEXT PRIMARY KEY,
	app_id TEXT NOT NULL REFERENCES app(id) ON DELETE CASCADE,
	label TEXT NOT NULL,
	enc_token_hash BLOB,
	created_at INTEGER NOT NULL,
	-- NO ACTION: refuse to delete an instance_usage_plan while an instance is
	-- assigned to it; tolerant of the app-delete cascade (both rows go together).
	usage_plan_id TEXT REFERENCES instance_usage_plan(id) ON DELETE NO ACTION
);

CREATE TABLE IF NOT EXISTS app_credential (
	app_id TEXT NOT NULL REFERENCES app(id) ON DELETE CASCADE,
	transport TEXT NOT NULL,
	enc_blob BLOB NOT NULL,
	PRIMARY KEY (app_id, transport)
);

CREATE TABLE IF NOT EXISTS app_usage_plan (
	id TEXT PRIMARY KEY,
	app_id TEXT REFERENCES app(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	is_default BOOLEAN NOT NULL DEFAULT FALSE,
	created_at INTEGER NOT NULL,
	push_per_min INTEGER NOT NULL DEFAULT 0,
	push_daily_quota INTEGER NOT NULL DEFAULT 0,
	register_per_min INTEGER NOT NULL DEFAULT 0,
	register_burst INTEGER NOT NULL DEFAULT 0,
	register_ip_per_min INTEGER NOT NULL DEFAULT 0,
	register_ip_burst INTEGER NOT NULL DEFAULT 0,
	subscribe_per_min INTEGER NOT NULL DEFAULT 0,
	subscribe_burst INTEGER NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS app_usage_plan_uidx_app_id ON app_usage_plan(app_id) WHERE app_id IS NOT NULL;
-- At most one global default app plan (seeded/synced from config).
CREATE UNIQUE INDEX IF NOT EXISTS app_usage_plan_uidx_is_default ON app_usage_plan(is_default) WHERE is_default = TRUE;
CREATE UNIQUE INDEX IF NOT EXISTS app_usage_plan_uidx_name ON app_usage_plan(name) WHERE app_id IS NULL;

CREATE TABLE IF NOT EXISTS instance_usage_plan (
	id TEXT PRIMARY KEY,
	app_id TEXT NOT NULL REFERENCES app(id) ON DELETE CASCADE,
	instance_id TEXT REFERENCES instance(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	is_default BOOLEAN NOT NULL DEFAULT FALSE,
	created_at INTEGER NOT NULL,
	push_per_min INTEGER NOT NULL DEFAULT 0,
	push_burst INTEGER NOT NULL DEFAULT 0,
	push_daily_quota INTEGER NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS instance_usage_plan_uidx_instance_id ON instance_usage_plan(instance_id) WHERE instance_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS instance_usage_plan_uidx_app_id_name ON instance_usage_plan(app_id, name) WHERE instance_id IS NULL;
-- Exactly one default (app-wide) instance plan per app. Seeded at app creation.
CREATE UNIQUE INDEX IF NOT EXISTS instance_usage_plan_uidx_app_id ON instance_usage_plan(app_id) WHERE is_default = TRUE;

-- Durable daily-quota counters (write-behind backup of the in-memory Quota).
-- scope is namespaced: "app:<id>" or "inst:<id>"; date is UTC "2006-01-02".
-- Rows are ephemeral counters aged out by the janitor, not relational state.
CREATE TABLE IF NOT EXISTS usage_counter (
	scope TEXT NOT NULL,
	date DATE NOT NULL,
	count INTEGER NOT NULL,
	PRIMARY KEY (scope, date)
);

CREATE TABLE IF NOT EXISTS settings (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	retry_max_attempts INTEGER NOT NULL DEFAULT 3,
	retry_base_backoff TEXT NOT NULL DEFAULT '200ms',
	retry_max_backoff TEXT NOT NULL DEFAULT '5s',
	auth_fail_ip_per_min INTEGER NOT NULL DEFAULT 5,
	auth_fail_ip_burst INTEGER NOT NULL DEFAULT 5
);
INSERT INTO settings (id) VALUES (1) ON CONFLICT(id) DO NOTHING;
