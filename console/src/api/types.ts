export interface AppView {
  id: string;
  name: string;
  is_public: boolean;
  key_version: number;
  min_key_version: number;
  usage_plan_id: string | null;
  turnstile_site_key: string | null;
  created_at: number; // unix seconds
}

export interface InstanceView {
  id: string;
  app_id: string;
  label: string;
  usage_plan_id: string | null;
  created_at: number;
}

export interface AppUsagePlanView {
  id: string;
  app_id: string | null; // null = shared plan
  name: string;
  is_default: boolean;
  push_per_min: number;
  push_daily_quota: number;
  register_per_min: number;
  register_burst: number;
  register_ip_per_min: number;
  register_ip_burst: number;
  subscribe_per_min: number;
  subscribe_burst: number;
}

// Backoffs are Go duration strings (e.g. "200ms", "5s").
export interface SettingsView {
  retry_max_attempts: number;
  retry_base_backoff: string;
  retry_max_backoff: string;
  auth_fail_ip_per_min: number;
  auth_fail_ip_burst: number;
}

export interface InstanceUsagePlanView {
  id: string;
  instance_id: string | null; // null = shared across the app's instances
  name: string;
  is_default: boolean;
  push_per_min: number;
  push_burst: number;
  push_daily_quota: number;
}

export interface AppCreated {
  id: string;
  name: string;
  token: string; // pat_ app token, shown exactly once
}

export interface InstanceCreated {
  id: string;
  token: string; // pit_ instance token, shown exactly once
}

export interface RotatedToken {
  token: string;
}

export interface TransportsView {
  transports: string[]; // subset of apns | fcm | webpush
}

export interface TurnstileView {
  configured: boolean;
  site_key: string | null;
}

export interface StatsView {
  date: string; // YYYY-MM-DD (UTC)
  total_apps: number;
  total_instances: number;
  total_pushes: number;
  pushes_by_app: Record<string, number>;
  pushes_by_instance: Record<string, number>;
}

export interface AppStatsView {
  date: string; // YYYY-MM-DD (UTC)
  total_instances: number;
  total_pushes: number;
  pushes_by_instance: Record<string, number>;
}

export interface ApnsCreds {
  key_p8: string;
  key_id: string;
  team_id: string;
  topic: string;
}

export interface FcmCreds {
  service_account_json: string;
}

export interface WebPushCreds {
  vapid_private_key: string;
  subject: string;
}
