/***Statement***/
ALTER TABLE user_activations ADD COLUMN redirect_organization_slug TEXT;
/***Statement***/
ALTER TABLE user_activations ADD COLUMN redirect_workspace_slug TEXT;
/***Statement***/
ALTER TABLE user_activations ADD COLUMN redirect_application_client_id TEXT;
