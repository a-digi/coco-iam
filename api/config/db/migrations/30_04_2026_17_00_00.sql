/***Statement***/
-- Add oauth_client_id to application_login_settings so admins can pin
-- the dispatch target to one of the application's registered OAuth
-- clients. When set, SaveSettings validates that redirect_url is one
-- of the client's registered redirect_uris. NULL means the redirect
-- URL is manually specified (existing behaviour).
ALTER TABLE application_login_settings ADD COLUMN oauth_client_id TEXT;
