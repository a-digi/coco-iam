package validator

import (
	jwt_token "github.com/a-digi/coco-iam/src/auth/security/jwt"
	"github.com/a-digi/coco-server/server/request"
)

func VerifySuperAdminPrivilege(reqCtx request.RequestContext) bool {
	tokenPayload, err := jwt_token.CreateJWTTokenFromHeaders(reqCtx.GetRequest().Header)
	if err != nil {
		return false
	}

	hasSuperAdminScope := false
	for _, scopeStr := range tokenPayload.Scopes {
		if jwt_token.Scope(scopeStr) == jwt_token.SuperAdmin {
			hasSuperAdminScope = true
			break
		}
	}

	if !hasSuperAdminScope {
		return false
	}

	return true
}
