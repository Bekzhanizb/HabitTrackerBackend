// middleware/csrf.go
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/csrf"
)

func CSRFMiddleware(authKey []byte, skipPaths ...string) gin.HandlerFunc {
	skip := make(map[string]bool)
	for _, path := range skipPaths {
		skip[path] = true
	}

	gorillaCSRF := csrf.Protect(
		authKey,
		csrf.Secure(false),
		csrf.HttpOnly(true),
		csrf.Path("/"),
		csrf.SameSite(csrf.SameSiteLaxMode),
		csrf.ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error": "CSRF token validation failed"}`))
		})),
	)

	return func(c *gin.Context) {
		if skip[c.Request.URL.Path] {
			c.Next()
			return
		}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c.Next()
		})

		gorillaCSRF(handler).ServeHTTP(c.Writer, c.Request)
	}
}

func GetCSRFToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := csrf.Token(c.Request)

		c.Header("X-CSRF-Token", token)

		c.JSON(http.StatusOK, gin.H{
			"csrfToken": token,
		})
	}
}
