/***Statement***/
-- Per-application login page gradient background. Admins now have
-- three precedences for the background: image (highest), gradient,
-- solid color (lowest). Empty gradient fields mean "no gradient" —
-- the public render falls through to the solid color.
ALTER TABLE application_login_settings ADD COLUMN background_gradient_from TEXT NOT NULL DEFAULT '';
/***Statement***/
ALTER TABLE application_login_settings ADD COLUMN background_gradient_to TEXT NOT NULL DEFAULT '';
/***Statement***/
ALTER TABLE application_login_settings ADD COLUMN background_gradient_angle INTEGER NOT NULL DEFAULT 135;
