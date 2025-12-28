// middleware/csrf.go
package middleware

import (
	"net/http"

	"github.com/Bekzhanizb/HabitTrackerBackend/utils"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/csrf"
	"go.uber.org/zap"
)

func CSRFMiddleware(authKey []byte, skipPaths ...string) gin.HandlerFunc {
	skip := make(map[string]bool)
	for _, path := range skipPaths {
		skip[path] = true
	}

	gorillaCSRF := csrf.Protect(
		authKey,
		csrf.Secure(false), // установите true в production!
		csrf.HttpOnly(true),
		csrf.Path("/"),
		csrf.SameSite(csrf.SameSiteLaxMode),
		csrf.RequestHeader("X-CSRF-Token"),
		csrf.FieldName("csrf_token"),
		csrf.ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			utils.Logger.Error("CSRF validation failed",
				zap.String("path", r.URL.Path),
				zap.String("method", r.Method),
				zap.String("header_csrf", r.Header.Get("X-CSRF-Token")),
				zap.String("cookie_csrf", r.Header.Get("Cookie")))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error": "CSRF token validation failed"}`))
		})),
	)

	return func(c *gin.Context) {
		utils.Logger.Info("CSRF middleware",
			zap.String("path", c.Request.URL.Path),
			zap.String("method", c.Request.Method),
			zap.Bool("skip", skip[c.Request.URL.Path]))

		if skip[c.Request.URL.Path] {
			c.Next()
			return
		}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c.Request = r
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
			"csrf_token": token,
		})
	}
}
