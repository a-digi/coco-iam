/***Statement***/
-- Speeds up "count failed attempts for this IP within the last N
-- seconds" - the failed-login ban-rule check runs on every single
-- failed login, so it must stay cheap regardless of how large this
-- table grows before the archiver rotates it out. See
-- plan/login-ban-rules/plan.md.
CREATE INDEX IF NOT EXISTS application_login_attempts_ip_success_idx ON application_login_attempts (ip, success, created_at DESC);
