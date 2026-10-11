package auth

import (
	"errors"
	"log/slog"
	"net/http"
)

// RequireHTTP lets a request through to next only when its bearer token carries the permission.
func RequireHTTP(a *Authenticator, log *slog.Logger, permission string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r.Header.Get("Authorization"))

		if !ok {
			unauthorized(w)
			return
		}

		principal, err := a.Authenticate(r.Context(), token)

		if errors.Is(err, ErrInvalidToken) {
			unauthorized(w)
			return
		}

		if err != nil {
			log.ErrorContext(r.Context(), "failed to authenticate request", slog.Any("error", err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

			return
		}

		if !principal.Can(permission) {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r.WithContext(NewContext(r.Context(), principal)))
	})
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
}
