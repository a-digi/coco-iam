package handler

import (
	"net/http"
	"time"

	"github.com/a-digi/coco-iam/config"
	user_acl_repo "github.com/a-digi/coco-iam/src/admin/acl/repository"
	mfa_entity "github.com/a-digi/coco-iam/src/admin/mfa/entity"
	mfa_persistent "github.com/a-digi/coco-iam/src/admin/mfa/repository/persistent"
	mfa_query "github.com/a-digi/coco-iam/src/admin/mfa/repository/query"
	"github.com/a-digi/coco-iam/src/admin/security/loginlog"
	"github.com/a-digi/coco-iam/src/auth/crypto/bcrypt"
	"github.com/a-digi/coco-iam/src/auth/crypto/secretbox"
	"github.com/a-digi/coco-iam/src/auth/mfa/totp"
	"github.com/a-digi/coco-iam/src/auth/oauth"
	oauth_lib "github.com/a-digi/coco-oauth/oauth"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// maxFailedAttempts is how many consecutive wrong codes (TOTP or
// recovery) are tolerated before the account is temporarily locked
// out of verify-mfa — this is the actual brute-force throttle for
// the login path (unlike /confirm, which is reached only by an
// already-fully-authenticated session and isn't the attack surface).
const maxFailedAttempts = 5

// lockoutDuration is how long verify-mfa is refused once
// maxFailedAttempts is reached.
const lockoutDuration = 15 * time.Minute

type VerifyMfaHandler struct{}

// @Summary     Verify TOTP code and complete login
// @Description Exchanges a pending mfa_token (scope system:mfa_required, minted by /admin/oauth/authenticate) plus a valid TOTP code or unused recovery code for a full-scope access token.
// @Tags        auth
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body mfa_entity.MfaCodeRequest true "6-digit code, or an unused recovery code"
// @Success     200 {object} entity.LoginSuccess
// @Failure     400 {object} response.ErrorBody "invalid code"
// @Failure     401 {object} response.ErrorBody "missing/invalid/expired mfa_token"
// @Failure     403 {object} response.ErrorBody "no scopes assigned to this account"
// @Failure     429 {object} response.ErrorBody "too many failed attempts — temporarily locked out"
// @Failure     500 {object} response.ErrorBody
// @Router      /admin/oauth/verify-mfa [post]
func (h *VerifyMfaHandler) ServeHTTP(reqCtx request.RequestContext) {
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
	if body.Code == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "code is required")
		return
	}

	q := mfa_query.NewAdminMfaQueryRepo(db)
	p := mfa_persistent.NewAdminMfaPersistentRepo(db)

	row, err := q.FindByAdminUserID(adminUserID)
	if err != nil || !row.IsEnabled {
		// Shouldn't happen — login only mints this token when MFA is
		// enabled — but a disable-in-another-tab race is possible.
		response.ErrorResponse(w, http.StatusUnauthorized, "MFA is not enabled for this account")
		return
	}

	if row.LockedUntil != nil && time.Now().UTC().Before(*row.LockedUntil) {
		response.ErrorResponse(w, http.StatusTooManyRequests, "too many failed attempts — try again later")
		return
	}

	if !verifyCode(q, p, adminUserID, row.SecretEnc, body.Code) {
		recordFailureAndMaybeLock(p, adminUserID, row.FailedAttempts)
		loginlog.Record(reqCtx, adminUserID, fetchAdminUsername(db, adminUserID), false, "mfa_failed")
		response.ErrorResponse(w, http.StatusBadRequest, "invalid code")
		return
	}

	if err := p.ResetFailedAttempts(adminUserID); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	manager := reqCtx.GetDI().GetDatabaseManager()
	aclRepo := user_acl_repo.NewUserAclRepository(manager)
	scopes, err := aclRepo.FindUserScopes(adminUserID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to get user scopes")
		return
	}
	if len(scopes) == 0 {
		response.ErrorResponse(w, http.StatusForbidden, "No access rights have been assigned to your account. Please contact your administrator.")
		return
	}

	cfgBytes, err := config.ReadConfigFile("config.json")
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "auth config error")
		return
	}
	cfg, err := oauth_lib.LoadAuthConfigFromBytes(cfgBytes)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "auth config error")
		return
	}

	tokenResp, err := oauth.IssueToken(cfg, adminUserID, scopes)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "token signing failed")
		return
	}
	loginlog.Record(reqCtx, adminUserID, fetchAdminUsername(db, adminUserID), true, "")
	response.SuccessResponse(w, http.StatusOK, tokenResp)
}

// verifyCode tries the submitted value as a TOTP code first, then as
// an unused recovery code — consuming the matching recovery code so
// it can never be replayed. Returns false if neither matches.
func verifyCode(q *mfa_query.AdminMfaQueryRepo, p *mfa_persistent.AdminMfaPersistentRepo, adminUserID, secretEnc, code string) bool {
	secret, err := secretbox.Decrypt(secretEnc)
	if err == nil && totp.Validate(secret, code, 1) {
		return true
	}

	codes, err := q.FindUnusedRecoveryCodes(adminUserID)
	if err != nil {
		return false
	}
	for _, rc := range codes {
		if bcrypt.VerifyPassword(rc.CodeHash, code) == nil {
			_ = p.MarkRecoveryCodeUsed(rc.ID)
			return true
		}
	}
	return false
}

// recordFailureAndMaybeLock increments the failure counter and, if
// this failure reaches maxFailedAttempts, sets a lockUntil
// lockoutDuration from now. previousCount is the failed_attempts
// value already on the row before this attempt.
func recordFailureAndMaybeLock(p *mfa_persistent.AdminMfaPersistentRepo, adminUserID string, previousCount int) {
	var lockUntil *time.Time
	if previousCount+1 >= maxFailedAttempts {
		t := time.Now().UTC().Add(lockoutDuration)
		lockUntil = &t
	}
	_ = p.RecordFailedAttempt(adminUserID, lockUntil)
}
