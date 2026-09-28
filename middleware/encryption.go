// Package middleware 中的 encryption.go 实现 APP 接口的对称加密 + 签名校验。
//
// 客户端约定:
//
//	Header:
//	  X-App-Id     客户端标识,服务端据此查找 secret
//	  X-Timestamp  毫秒时间戳
//	  X-Nonce      每次请求唯一的随机串 (建议 16 字节的 base64)
//	  X-Sign       base64( HMAC-SHA256(secret,
//	                 timestamp + "\n" + nonce + "\n" + METHOD + "\n" + path + "\n" + rawBody ) )
//	Body (POST/PUT/PATCH):
//	  { "data": "<base64( iv(12) || ciphertext || tag(16) )>" }
//	  明文为原始 JSON,使用 AES-256-GCM 加密,key = SHA-256(secret)[:32]。
//
// 服务端仅解密请求体,响应体保持明文 (按约定关闭双向加密)。
package middleware

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"yy-kitchen-logic/config"
)

const (
	HeaderAppID     = "X-App-Id"
	HeaderTimestamp = "X-Timestamp"
	HeaderNonce     = "X-Nonce"
	HeaderSign      = "X-Sign"

	encryptedBodyProtectedPathPrefix = "/api/app/v1/"
)

// EncryptedEnvelope 是加密请求体的最外层结构。
type EncryptedEnvelope struct {
	Data string `json:"data"`
}

// NewEncryptionMiddleware 返回一个 gin 中间件:校验签名 -> 校验时间戳/nonce ->
// 解密 body -> 用解密后的明文替换 c.Request.Body,后续 Huma / handler 无感知。
// 当 cfg.Enabled=false 或路径不在保护范围内时直接放行。
func NewEncryptionMiddleware(cfg config.EncryptionConfig) gin.HandlerFunc {
	if !cfg.Enabled {
		return func(c *gin.Context) { c.Next() }
	}

	secrets := make(map[string]string, len(cfg.Apps))
	for _, app := range cfg.Apps {
		if app.ID == "" || app.Secret == "" {
			continue
		}
		secrets[app.ID] = app.Secret
	}

	skew := time.Duration(cfg.TimestampSkewSeconds) * time.Second
	if skew <= 0 {
		skew = 5 * time.Minute
	}
	nonceTTL := time.Duration(cfg.NonceTTLSeconds) * time.Second
	if nonceTTL <= 0 {
		nonceTTL = 2 * skew
	}
	store := newNonceStore(nonceTTL)

	return func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, encryptedBodyProtectedPathPrefix) {
			c.Next()
			return
		}

		appID := c.GetHeader(HeaderAppID)
		tsStr := c.GetHeader(HeaderTimestamp)
		nonce := c.GetHeader(HeaderNonce)
		sign := c.GetHeader(HeaderSign)
		if appID == "" || tsStr == "" || nonce == "" || sign == "" {
			abortEncryptionErr(c, "missing encryption headers")
			return
		}

		secret, ok := secrets[appID]
		if !ok {
			abortEncryptionErr(c, "unknown app id")
			return
		}

		tsMs, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			abortEncryptionErr(c, "invalid timestamp")
			return
		}
		ts := time.UnixMilli(tsMs)
		if diff := time.Since(ts); diff > skew || diff < -skew {
			abortEncryptionErr(c, "timestamp out of window")
			return
		}

		// 读取原始 body,签名基于密文 body,校验完再解密。
		var rawBody []byte
		if c.Request.Body != nil {
			rawBody, err = io.ReadAll(c.Request.Body)
			if err != nil {
				abortEncryptionErr(c, "read body failed")
				return
			}
			_ = c.Request.Body.Close()
		}

		if !verifySignature(secret, tsStr, nonce, c.Request.Method, c.Request.URL.Path, rawBody, sign) {
			abortEncryptionErr(c, "invalid signature")
			return
		}

		// 签名校验通过后再登记 nonce,避免为无效请求填库。
		if !store.checkAndSet(appID+"|"+nonce, ts.Add(nonceTTL)) {
			abortEncryptionErr(c, "duplicate nonce")
			return
		}

		// 只有当请求包含 body 时才解密。GET/DELETE 等无 body 请求跳过。
		if len(bytes.TrimSpace(rawBody)) > 0 {
			plaintext, err := decryptEnvelope(secret, rawBody)
			if err != nil {
				abortEncryptionErr(c, "decrypt failed: "+err.Error())
				return
			}
			c.Request.Body = io.NopCloser(bytes.NewReader(plaintext))
			c.Request.ContentLength = int64(len(plaintext))
			c.Request.Header.Set("Content-Type", "application/json")
		}

		c.Next()
	}
}

func abortEncryptionErr(c *gin.Context, msg string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": msg})
}

func verifySignature(secret, ts, nonce, method, path string, rawBody []byte, provided string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts))
	mac.Write([]byte{'\n'})
	mac.Write([]byte(nonce))
	mac.Write([]byte{'\n'})
	mac.Write([]byte(strings.ToUpper(method)))
	mac.Write([]byte{'\n'})
	mac.Write([]byte(path))
	mac.Write([]byte{'\n'})
	mac.Write(rawBody)
	expected := mac.Sum(nil)

	got, err := base64.StdEncoding.DecodeString(provided)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(expected, got) == 1
}

func decryptEnvelope(secret string, rawBody []byte) ([]byte, error) {
	var env EncryptedEnvelope
	if err := json.Unmarshal(rawBody, &env); err != nil {
		return nil, errors.New("body not encrypted envelope")
	}
	if env.Data == "" {
		return nil, errors.New("empty data field")
	}
	blob, err := base64.StdEncoding.DecodeString(env.Data)
	if err != nil {
		return nil, errors.New("data not base64")
	}

	sum := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(blob) < gcm.NonceSize()+gcm.Overhead() {
		return nil, errors.New("ciphertext too short")
	}
	iv := blob[:gcm.NonceSize()]
	ct := blob[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, iv, ct, nil)
	if err != nil {
		return nil, errors.New("gcm auth failed")
	}
	return plaintext, nil
}

// nonceStore 用 sync.Map 保存 nonce -> 过期时间。每 TTL/2 触发一次懒清理。
type nonceStore struct {
	m        sync.Map
	ttl      time.Duration
	lastScan time.Time
	scanMu   sync.Mutex
}

func newNonceStore(ttl time.Duration) *nonceStore {
	return &nonceStore{ttl: ttl, lastScan: time.Now()}
}

// checkAndSet 若 key 未见过则登记并返回 true;若已经存在(且未过期)返回 false。
func (s *nonceStore) checkAndSet(key string, expireAt time.Time) bool {
	s.maybeSweep()
	if prev, loaded := s.m.LoadOrStore(key, expireAt); loaded {
		if exp, ok := prev.(time.Time); ok && time.Now().Before(exp) {
			return false
		}
		s.m.Store(key, expireAt)
	}
	return true
}

func (s *nonceStore) maybeSweep() {
	s.scanMu.Lock()
	if time.Since(s.lastScan) < s.ttl/2 {
		s.scanMu.Unlock()
		return
	}
	s.lastScan = time.Now()
	s.scanMu.Unlock()

	now := time.Now()
	s.m.Range(func(k, v any) bool {
		if exp, ok := v.(time.Time); ok && now.After(exp) {
			s.m.Delete(k)
		}
		return true
	})
}
