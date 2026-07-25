// Package handler serves the admin self-service TOTP MFA endpoints
// under /api/v1/admin/users/me/mfa* — all gated by the admin:me scope,
// same as every other .../users/me endpoint. The login-time
// verify-mfa handler lives alongside these (verify_mfa_handler.go)
// since it shares the lockout policy and repositories, even though
// it's reached via a different (system:mfa_required) scope.
package handler

import (
	"database/sql"
	"errors"
	"net/http"

	mfa_entity "github.com/a-digi/coco-iam/src/admin/mfa/entity"
	mfa_persistent "github.com/a-digi/coco-iam/src/admin/mfa/repository/persistent"
	mfa_query "github.com/a-digi/coco-iam/src/admin/mfa/repository/query"
	"github.com/a-digi/coco-iam/src/auth/crypto/bcrypt"
	"github.com/a-digi/coco-iam/src/auth/crypto/secretbox"
	auth_db "github.com/a-digi/coco-iam/src/auth/database"
	auth_query "github.com/a-digi/coco-iam/src/auth/database/repository/query"
	"github.com/a-digi/coco-iam/src/auth/mfa/totp"
	jwt_token "github.com/a-digi/coco-iam/src/auth/security/jwt"
	"github.com/a-digi/coco-oauth/oauth"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
	"github.com/google/uuid"
)

// issuer is the fixed, recognizable name every enrolled authenticator
// app shows next to the account label.
const issuer = "coco-iam"

// recoveryCodeCount is how many single-use codes are issued on
// /confirm and on every regenerate — invalidates any previous set.
const recoveryCodeCount = 8

// -- Status ---------------------------------------------------------------

type MfaStatusHandler struct{}

// @Summary     Get current admin's MFA status
// @Tags        admin-me
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} mfa_entity.MfaStatusSuccess
// @Failure     401 {object} response.ErrorBody
// @Failure     500 {object} response.ErrorBody
// @Router      /admin/users/me/mfa [get]
func (h *MfaStatusHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	adminUserID, db, ok := resolveCaller(reqCtx, w)
	if !ok {
		return
	}

	q := mfa_query.NewAdminMfaQueryRepo(db)
	row, err := q.FindByAdminUserID(adminUserID)
	if err != nil && !errors.Is(err, mfa_query.ErrNotFound) {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	out := mfa_entity.MfaStatus{}
	if row != nil {
		out.Enabled = row.IsEnabled
		if row.EnrolledAt != nil {
			out.EnrolledAt = row.EnrolledAt.Format("2006-01-02T15:04:05Z07:00")
		}
		n, err := q.CountUnusedRecoveryCodes(adminUserID)
		if err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		out.RecoveryCodesRemaining = n
	}
	response.SuccessResponse(w, http.StatusOK, out)
}

// -- Enroll -----------------------------------------------------------------

type MfaEnrollHandler struct{}

// @Summary     Start TOTP enrollment for the current admin
// @Description Generates a new secret and provisioning URI. Not enabled until /confirm succeeds. Calling again before confirming replaces the pending secret.
// @Tags        admin-me
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} mfa_entity.MfaEnrollSuccess
// @Failure     401 {object} response.ErrorBody
// @Failure     500 {object} response.ErrorBody
// @Router      /admin/users/me/mfa/enroll [post]
func (h *MfaEnrollHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	adminUserID, db, ok := resolveCaller(reqCtx, w)
	if !ok {
		return
	}

	email, err := fetchAdminEmail(db, adminUserID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	secret, err := totp.GenerateSecret()
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to generate secret: "+err.Error())
		return
	}
	secretEnc, err := secretbox.Encrypt(secret)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to encrypt secret: "+err.Error())
		return
	}

	p := mfa_persistent.NewAdminMfaPersistentRepo(db)
	if err := p.UpsertPendingSecret(adminUserID, secretEnc); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.SuccessResponse(w, http.StatusOK, mfa_entity.MfaEnrollResponse{
		Secret:          secret,
		ProvisioningURI: totp.ProvisioningURI(secret, issuer, email),
	})
}

// -- Confirm ------------------------------------------------------------

type MfaConfirmHandler struct{}

// @Summary     Confirm TOTP enrollment
// @Tags        admin-me
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body mfa_entity.MfaCodeRequest true "6-digit code from the authenticator app"
// @Success     200 {object} mfa_entity.MfaRecoveryCodesSuccess
// @Failure     400 {object} response.ErrorBody "invalid code, or no pending enrollment"
// @Failure     401 {object} response.ErrorBody
// @Failure     500 {object} response.ErrorBody
// @Router      /admin/users/me/mfa/confirm [post]
func (h *MfaConfirmHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	adminUserID, db, ok := resolveCaller(reqCtx, w)
	if !ok {
		return
	}

	var body mfa_entity.MfaCodeRequest
	if err := reqCtx.BindJSON(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	q := mfa_query.NewAdminMfaQueryRepo(db)
	row, err := q.FindByAdminUserID(adminUserID)
	if err != nil {
		if errors.Is(err, mfa_query.ErrNotFound) {
			response.ErrorResponse(w, http.StatusBadRequest, "no pending enrollment — call /enroll first")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	secret, err := secretbox.Decrypt(row.SecretEnc)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to decrypt secret: "+err.Error())
		return
	}
	if !totp.Validate(secret, body.Code, 1) {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid code")
		return
	}

	p := mfa_persistent.NewAdminMfaPersistentRepo(db)
	if err := p.Confirm(adminUserID); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	codes, err := issueRecoveryCodes(p, adminUserID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, mfa_entity.MfaRecoveryCodesResponse{RecoveryCodes: codes})
}

// -- Disable --------------------------------------------------------------

type MfaDisableHandler struct{}

// @Summary     Disable TOTP for the current admin
// @Tags        admin-me
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body mfa_entity.MfaDisableRequest true "current password, re-verified"
// @Success     200 {object} mfa_entity.StatusSuccess
// @Failure     400 {object} response.ErrorBody "wrong password"
// @Failure     401 {object} response.ErrorBody
// @Failure     500 {object} response.ErrorBody
// @Router      /admin/users/me/mfa [delete]
func (h *MfaDisableHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	adminUserID, db, ok := resolveCaller(reqCtx, w)
	if !ok {
		return
	}

	if !verifyCurrentPassword(reqCtx, adminUserID) {
		return
	}

	p := mfa_persistent.NewAdminMfaPersistentRepo(db)
	if err := p.Disable(adminUserID); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, mfa_entity.StatusResponse{Status: "disabled"})
}

// -- Recovery codes regenerate ----------------------------------------------

type MfaRecoveryCodesRegenerateHandler struct{}

// @Summary     Regenerate recovery codes
// @Tags        admin-me
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body mfa_entity.MfaDisableRequest true "current password, re-verified"
// @Success     200 {object} mfa_entity.MfaRecoveryCodesSuccess
// @Failure     400 {object} response.ErrorBody "wrong password, or MFA not enabled"
// @Failure     401 {object} response.ErrorBody
// @Failure     500 {object} response.ErrorBody
// @Router      /admin/users/me/mfa/recovery-codes [post]
func (h *MfaRecoveryCodesRegenerateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	adminUserID, db, ok := resolveCaller(reqCtx, w)
	if !ok {
		return
	}

	if !verifyCurrentPassword(reqCtx, adminUserID) {
		return
	}

	q := mfa_query.NewAdminMfaQueryRepo(db)
	row, err := q.FindByAdminUserID(adminUserID)
	if err != nil {
		if errors.Is(err, mfa_query.ErrNotFound) {
			response.ErrorResponse(w, http.StatusBadRequest, "MFA is not enabled")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !row.IsEnabled {
		response.ErrorResponse(w, http.StatusBadRequest, "MFA is not enabled")
		return
	}

	p := mfa_persistent.NewAdminMfaPersistentRepo(db)
	codes, err := issueRecoveryCodes(p, adminUserID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, mfa_entity.MfaRecoveryCodesResponse{RecoveryCodes: codes})
}

// -- shared helpers -----------------------------------------------------

// resolveCaller extracts the admin id from the bearer token and
// resolves the main-DB handle, writing a response and returning
// ok=false on any failure so handlers can short-circuit in one line.
func resolveCaller(reqCtx request.RequestContext, w http.ResponseWriter) (adminUserID string, db *sql.DB, ok bool) {
	r := reqCtx.GetRequest()
	adminUserID, valid := subjectFromBearer(r.Header.Get("Authorization"))
	if !valid {
		response.ErrorResponse(w, http.StatusUnauthorized, "unauthorized")
		return "", nil, false
	}
	manager := reqCtx.GetDI().GetDatabaseManager()
	if manager == nil || manager.Connector == nil || manager.Connector.DB == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "database manager not available")
		return "", nil, false
	}
	return adminUserID, manager.Connector.DB, true
}

func subjectFromBearer(header string) (string, bool) {
	if header == "" {
		return "", false
	}
	token, err := oauth.ExtractBearer(header)
	if err != nil {
		return "", false
	}
	userID, err := jwt_token.ParseJWTSubject(token)
	if err != nil || userID == "" {
		return "", false
	}
	return userID, true
}

func fetchAdminEmail(db *sql.DB, adminUserID string) (string, error) {
	var email string
	err := db.QueryRow(`SELECT email FROM admin_users WHERE id = ? LIMIT 1`, adminUserID).Scan(&email)
	if err != nil {
		return "", err
	}
	return email, nil
}

// verifyCurrentPassword re-checks the caller's current password
// against admin_auth_password, writing the appropriate error
// response and returning false on any failure — a hijacked session
// shouldn't be able to silently disable MFA or mint new recovery
// codes.
func verifyCurrentPassword(reqCtx request.RequestContext, adminUserID string) bool {
	w := reqCtx.GetWriter()
	var body mfa_entity.MfaDisableRequest
	if err := reqCtx.BindJSON(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	if body.Password == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "password is required")
		return false
	}

	manager := reqCtx.GetDI().GetDatabaseManager()
	pwrepo := auth_query.NewAdminPasswordQueryRepository(manager)
	authenticator := auth_db.NewPasswordAuthenticator(pwrepo)
	ok, err := authenticator.Verify(adminUserID, body.Password)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "authentication error")
		return false
	}
	if !ok {
		response.ErrorResponse(w, http.StatusBadRequest, "wrong password")
		return false
	}
	return true
}

// issueRecoveryCodes generates a fresh set of recovery codes, hashes
// each with bcrypt, and atomically replaces whatever set previously
// existed — returning the plaintext codes for one-time display.
func issueRecoveryCodes(p *mfa_persistent.AdminMfaPersistentRepo, adminUserID string) ([]string, error) {
	codes, err := totp.GenerateRecoveryCodes(recoveryCodeCount)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(codes))
	hashes := make([]string, len(codes))
	for i, c := range codes {
		ids[i] = uuid.New().String()
		hash, err := bcrypt.HashPassword(c, bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		hashes[i] = hash
	}
	if err := p.ReplaceRecoveryCodes(adminUserID, ids, hashes); err != nil {
		return nil, err
	}
	return codes, nil
}
