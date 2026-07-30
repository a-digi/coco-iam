/***Statement***/
-- JSON snapshot of the country/ASN/ISP resolved for this attempt's IP
-- at record time (same format geoip.Info marshals to elsewhere in
-- this codebase). Never re-derived later - geoip.db keeps no
-- history, so a stored value must not silently change under an old
-- row. NULL means either the IP was loopback/private, GeoIP had no
-- coverage, or the row predates this column. See
-- plan/login-log-geoip/plan.md.
ALTER TABLE application_login_attempts ADD COLUMN geoip_info TEXT;
