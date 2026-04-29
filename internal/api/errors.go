package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func errResponse(code, message string) gin.H {
	return gin.H{
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	}
}

func abortErr(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, errResponse(code, message))
}

func jsonErr(c *gin.Context, status int, code, message string) {
	c.JSON(status, errResponse(code, message))
}

// Convenience wrappers for each HTTP status.

func err400(c *gin.Context, message string) {
	jsonErr(c, http.StatusBadRequest, "InvalidRequest", message)
}

func err400Missing(c *gin.Context, message string) {
	jsonErr(c, http.StatusBadRequest, "MissingRequiredProperty", message)
}

func err401(c *gin.Context, message string) {
	abortErr(c, http.StatusUnauthorized, "AuthenticationFailed", message)
}

func err404(c *gin.Context, message string) {
	jsonErr(c, http.StatusNotFound, "ResourceNotFound", message)
}

func err409(c *gin.Context, message string) {
	jsonErr(c, http.StatusConflict, "ResourceAlreadyExists", message)
}

func err500(c *gin.Context, message string) {
	jsonErr(c, http.StatusInternalServerError, "InternalServerError", message)
}
