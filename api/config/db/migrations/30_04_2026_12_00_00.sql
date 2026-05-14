/***Statement***/
ALTER TABLE applications ADD COLUMN registration_type TEXT NOT NULL DEFAULT 'legacy';
