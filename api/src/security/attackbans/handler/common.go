package handler

import (
	"database/sql"
	"net/http"

	"github.com/a-digi/coco-iam/config/di"
	"github.com/a-digi/coco-iam/src/security/attackbans"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// resolveMainDB returns the main database's *sql.DB — the same one
// attack_ban_rules, login_ban_rules, geoip_settings, and ipguard's
// ban/allowlist repos all live in.
func resolveMainDB(reqCtx request.RequestContext) (*sql.DB, bool) {
	w := reqCtx.GetWriter()
	bag, ok := reqCtx.GetDI().(*di.ContextBag)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return nil, false
	}
	manager := bag.GetDatabaseManager()
	if manager == nil || manager.Connector == nil || manager.Connector.DB == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "database manager not available")
		return nil, false
	}
	return manager.Connector.DB, true
}

// resolveSettingsQuery and resolveSettingsPersist construct
// attackbans.SettingsQueryRepo/SettingsPersistentRepo on demand against
// the main database — cheap, stateless wrappers, same convention as
// loginbans/handler/common.go's own resolveSettingsQuery/resolveSettingsPersist.
func resolveSettingsQuery(reqCtx request.RequestContext) (*attackbans.SettingsQueryRepo, bool) {
	db, ok := resolveMainDB(reqCtx)
	if !ok {
		return nil, false
	}
	return attackbans.NewSettingsQueryRepo(db), true
}

func resolveSettingsPersist(reqCtx request.RequestContext) (*attackbans.SettingsPersistentRepo, bool) {
	db, ok := resolveMainDB(reqCtx)
	if !ok {
		return nil, false
	}
	return attackbans.NewSettingsPersistentRepo(db), true
}

// toResponse converts attackbans.Settings into the wire shape.
func toResponse(s attackbans.Settings) SettingsResponse {
	return SettingsResponse{
		Enabled:       s.Enabled,
		Threshold:     s.Threshold,
		WindowSeconds: s.WindowSeconds,
		BanSeconds:    s.BanSeconds,
	}
}

// resolvedSettingsResponse loads the current settings and converts
// them to the wire shape — shared by GET and PUT (PUT re-reads rather
// than echoing the request back, so the response always reflects what
// was actually stored).
func resolvedSettingsResponse(query *attackbans.SettingsQueryRepo) (SettingsResponse, error) {
	settings, err := query.LoadSettings()
	if err != nil {
		return SettingsResponse{}, err
	}
	return toResponse(settings), nil
}
