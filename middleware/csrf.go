package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/csrf"
)

func CSRFProtection(authKey []byte) gin.HandlerFunc {
	csrfMiddleware := csrf.Protect(
		authKey,
		csrf.Secure(true),
		csrf.HttpOnly(true),
		csrf.Path("/"),
		csrf.ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error": "CSRF token validation failed"}`))
		})),
	)

	return func(c *gin.Context) {
		handler := csrfMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := csrf.Token(r)
			c.Set("csrf_token", token)
			c.Header("X-CSRF-Token", token)
			c.Next()
		}))
		handler.ServeHTTP(c.Writer, c.Request)
	}
}
