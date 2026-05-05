package api

import (
	"log"
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

// err500 logs the internal error server-side and returns a generic message to
// the client so that internal details (DB errors, stack traces, etc.) are
// never exposed in the response body.
func err500(c *gin.Context, err error) {
	log.Printf("[ERROR] internal server error: %v", err)
	jsonErr(c, http.StatusInternalServerError, "InternalServerError", "An internal server error occurred.")
}
