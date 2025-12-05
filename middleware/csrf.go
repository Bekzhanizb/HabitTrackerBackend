// middleware/csrf.go
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/csrf"
)

// CSRFMiddleware - middleware для защиты от CSRF
func CSRFMiddleware(authKey []byte, skipPaths ...string) gin.HandlerFunc {
	skip := make(map[string]bool)
	for _, path := range skipPaths {
		skip[path] = true
	}

	// Создаем стандартный middleware от gorilla/csrf
	gorillaCSRF := csrf.Protect(
		authKey,
		csrf.Secure(false), // false на localhost, true в проде
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
		// Пропускаем проверку для указанных путей
		if skip[c.Request.URL.Path] {
			c.Next()
			return
		}

		// Используем адаптер для Gin
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Передаем контекст дальше
			c.Next()
		})

		// Обертываем наш обработчик в CSRF middleware
		gorillaCSRF(handler).ServeHTTP(c.Writer, c.Request)
	}
}

// GetCSRFToken - обработчик для получения CSRF токена
func GetCSRFToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Получаем CSRF токен из контекста
		token := csrf.Token(c.Request)

		// Устанавливаем заголовок для клиента
		c.Header("X-CSRF-Token", token)

		// Отправляем токен в ответе
		c.JSON(http.StatusOK, gin.H{
			"csrfToken": token,
		})
	}
}
