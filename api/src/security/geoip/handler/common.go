package handler

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/a-digi/coco-iam/config/di"
	"github.com/a-digi/coco-iam/src/security/geoip"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// resolveBag returns the DI context bag, writing a 500 response and
// returning ok=false if it's not the expected type — matches the
// convention already used by every other admin handler in this
// codebase (e.g. api/src/admin/security/handler/common.go's
// resolveIPGuard).
func resolveBag(reqCtx request.RequestContext) (*di.ContextBag, bool) {
	w := reqCtx.GetWriter()
	bag, ok := reqCtx.GetDI().(*di.ContextBag)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return nil, false
	}
	return bag, true
}

// resolveMainDB returns the main database's *sql.DB — the same one
// ipguard's ban/allowlist repos and geoip_settings both live in.
func resolveMainDB(reqCtx request.RequestContext) (*sql.DB, bool) {
	w := reqCtx.GetWriter()
	bag, ok := resolveBag(reqCtx)
	if !ok {
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
// geoip.SettingsQueryRepo/SettingsPersistentRepo on demand against the
// main database — cheap, stateless wrappers, so there's no need to
// pre-construct and store them on ContextBag the way GeoIPManager
// (below) has to be, since Manager needs config-derived paths at
// construction time that aren't available per-request.
func resolveSettingsQuery(reqCtx request.RequestContext) (*geoip.SettingsQueryRepo, bool) {
	db, ok := resolveMainDB(reqCtx)
	if !ok {
		return nil, false
	}
	return geoip.NewSettingsQueryRepo(db), true
}

func resolveSettingsPersist(reqCtx request.RequestContext) (*geoip.SettingsPersistentRepo, bool) {
	db, ok := resolveMainDB(reqCtx)
	if !ok {
		return nil, false
	}
	return geoip.NewSettingsPersistentRepo(db), true
}

// resolveManager returns the shared geoip.Manager instance constructed
// once at boot (see config/routes/routes.go) — so Start/Stop actually
// control the real process rather than some unrelated instance.
func resolveManager(reqCtx request.RequestContext) (*geoip.Manager, bool) {
	w := reqCtx.GetWriter()
	bag, ok := resolveBag(reqCtx)
	if !ok {
		return nil, false
	}
	manager := bag.GetGeoIPManager()
	if manager == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "geoip manager not available")
		return nil, false
	}
	return manager, true
}

// resolvedSettingsResponse loads the current settings and merges them
// onto geoip.DefaultConfig() (the same merge SaveSettings/routes.go
// wiring uses at runtime), so GET/PUT both show the actual effective
// values — including config.json's own static defaults when nothing
// has been saved yet — rather than raw zero values.
func resolvedSettingsResponse(query *geoip.SettingsQueryRepo) (SettingsResponse, error) {
	settings, err := query.LoadSettings()
	if err != nil {
		return SettingsResponse{}, err
	}
	cfg := geoip.DefaultConfig().WithSettings(settings)
	resp := SettingsResponse{
		Enabled:              cfg.Enabled,
		MaxMindAccountID:     cfg.MaxMindAccountID,
		CheckIntervalSeconds: cfg.CheckIntervalSeconds,
		PullIntervalHours:    cfg.PullIntervalHours,
	}
	if cfg.MaxMindLicenseKey != "" {
		resp.MaxMindLicenseKeyMask = licenseKeyMask
	}
	return resp, nil
}

// resolveStatusResponse assembles the same StatusResponse
// StatusHandler returns — shared with SyncHandler so a successful
// sync reflects the current, still-valid counts/last-pulled time
// (nothing was deleted, a new pull was just requested) rather than
// blanking them out until the next 5s status poll. Writes an error
// response and returns ok=false on failure, matching every other
// resolve* helper's convention.
func resolveStatusResponse(reqCtx request.RequestContext, manager *geoip.Manager) (StatusResponse, bool) {
	w := reqCtx.GetWriter()
	status, err := manager.Status()
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return StatusResponse{}, false
	}

	query, ok := resolveSettingsQuery(reqCtx)
	if !ok {
		return StatusResponse{}, false
	}
	settings, err := query.LoadSettings()
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return StatusResponse{}, false
	}
	cfg := geoip.DefaultConfig().WithSettings(settings)

	resp := StatusResponse{
		Running:           status.Running,
		PID:               status.PID,
		Enabled:           cfg.Enabled,
		CountryRangeCount: status.CountryRangeCount,
		ASNRangeCount:     status.ASNRangeCount,
	}
	if !status.LastPulledAt.IsZero() {
		resp.LastPulledAt = status.LastPulledAt.UTC().Format(time.RFC3339)
	}
	return resp, true
}
