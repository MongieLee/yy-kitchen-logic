package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	uploadDir       = "uploads"
	maxUploadBytes  = 50 << 20 // 50 MiB
	publicURLPrefix = "/uploads"
)

var allowedExts = map[string]struct{}{
	".png": {}, ".jpg": {}, ".jpeg": {}, ".gif": {}, ".webp": {}, ".svg": {},
}

// RegisterUploadRoutes mounts a multipart upload endpoint and serves the
// uploaded files back over HTTP. Uploads live under ./uploads and are exposed
// at /uploads/<filename>.
func RegisterUploadRoutes(r *gin.Engine) {
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		panic(fmt.Errorf("create upload dir: %w", err))
	}
	r.Static(publicURLPrefix, "./"+uploadDir)

	r.POST("/api/admin/v1/upload", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes)
		fh, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 file 字段或超出大小限制 (50MB)"})
			return
		}

		ext := strings.ToLower(filepath.Ext(fh.Filename))
		if _, ok := allowedExts[ext]; !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的文件类型: " + ext})
			return
		}

		buf := make([]byte, 8)
		if _, err := rand.Read(buf); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		name := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), hex.EncodeToString(buf), ext)
		dst := filepath.Join(uploadDir, name)
		if err := c.SaveUploadedFile(fh, dst); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 只返回相对路径，避免把某次请求时的 IP/端口固化进数据库
		// (家用路由器重新分配 IP 后旧的绝对 URL 会全部 404)。
		c.JSON(http.StatusOK, gin.H{
			"path":     publicURLPrefix + "/" + name,
			"filename": name,
			"size":     fh.Size,
		})
	})
}
