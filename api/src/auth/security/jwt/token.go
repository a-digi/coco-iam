package jwt

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/a-digi/coco-oauth/oauth"
)

type TokenPayload struct {
	Aud    string   `json:"aud"`
	Exp    int64    `json:"exp"`
	Iss    string   `json:"iss"`
	Scope  string   `json:"scope"`
	Scopes []string `json:"-"`
	Sub    string   `json:"sub"`
}

func ParseJWT(token string) (*TokenPayload, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid jwt format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}

	var data TokenPayload
	err = json.Unmarshal(payload, &data)

	if data.Scope != "" {
		data.Scopes = strings.Split(data.Scope, " ")
	} else {
		data.Scopes = []string{}
	}

	return &data, err
}

func ParseJWTSubject(token string) (string, error) {
	data, err := ParseJWT(token)

	if err != nil {
		return "", err
	}

	if data.Sub != "" {
		return data.Sub, nil
	}

	return "", errors.New("sub claim not found")
}

func CreateJWTTokenFromHeaders(header http.Header) (*TokenPayload, error) {
	authHeader := header.Get("Authorization")
	if authHeader == "" {
		return nil, errors.New("unauthorized")
	}

	token, err := oauth.ExtractBearer(authHeader)
	if err != nil {
		return nil, errors.New("invalid token format")
	}

	return ParseJWT(token)
}
