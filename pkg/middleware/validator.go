package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
}

// Validator 请求验证中间件
func Validator() gin.HandlerFunc {
	return func(c *gin.Context) {
		var err error
		var obj interface{}

		// 根据Content-Type获取请求体
		contentType := c.GetHeader("Content-Type")
		if strings.Contains(contentType, "application/json") {
			if err = c.ShouldBindJSON(&obj); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    http.StatusBadRequest,
					"message": "无效的请求参数: " + err.Error(),
				})
				c.Abort()
				return
			}
		} else if strings.Contains(contentType, "application/x-www-form-urlencoded") {
			if err = c.ShouldBind(&obj); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    http.StatusBadRequest,
					"message": "无效的请求参数: " + err.Error(),
				})
				c.Abort()
				return
			}
		}

		// 验证结构体
		if obj != nil {
			if err = validate.Struct(obj); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    http.StatusBadRequest,
					"message": "参数验证失败: " + err.Error(),
				})
				c.Abort()
				return
			}
		}

		// 验证URL参数
		if err = validateQueryParams(c); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    http.StatusBadRequest,
				"message": "URL参数验证失败: " + err.Error(),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// validateQueryParams 验证URL参数
func validateQueryParams(c *gin.Context) error {
	query := c.Request.URL.Query()
	for _, values := range query {
		for _, value := range values {
			if err := validate.Var(value, "required"); err != nil {
				return err
			}
		}
	}
	return nil
}

// RegisterCustomValidator 注册自定义验证器
func RegisterCustomValidator(tag string, fn validator.Func) {
	validate.RegisterValidation(tag, fn)
}
