package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireRole проверяет, что роль текущего админа входит в список разрешённых.
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString("role")
		for _, r := range roles {
			if role == r {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": "Недостаточно прав"})
	}
}

// JWTMiddleware проверяет наличие и валидность access-токена в куке.
// При успехе устанавливает adminId и role в контекст запроса.
func JWTMiddleware(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, err := c.Cookie("access_token")
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Отсутствует токен авторизации"})
			return
		}

		claims, err := svc.parseToken(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Невалидный токен"})
			return
		}

		tokenType, _ := claims["type"].(string)
		if tokenType != "access" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Невалидный токен"})
			return
		}

		c.Set("adminId", int(claims["adminId"].(float64)))
		c.Set("role", claims["role"].(string))
		c.Set("adminLogin", claims["login"].(string))
		c.Next()
	}
}
