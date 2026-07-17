package httpserver

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
)

func authenticate(apiToken string, next http.Handler) http.Handler {
	expectedHash := sha256.Sum256([]byte(apiToken))

	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		authorizationHeaders := request.Header.Values("Authorization")
		authorization := ""
		if len(authorizationHeaders) == 1 {
			authorization = authorizationHeaders[0]
		}
		scheme, token, found := strings.Cut(authorization, " ")
		tokenHash := sha256.Sum256([]byte(token))
		tokenMatches := subtle.ConstantTimeCompare(expectedHash[:], tokenHash[:]) == 1
		isAuthorized := len(authorizationHeaders) == 1 &&
			found &&
			scheme == "Bearer" &&
			tokenMatches
		if !isAuthorized {
			response.Header().Set("WWW-Authenticate", "Bearer")
			writeError(response, http.StatusUnauthorized, "unauthorized")
			return
		}

		next.ServeHTTP(response, request)
	})
}
