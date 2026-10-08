package cardkey

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"
)

// VerifyResp 验证服务器响应签名：sig = RSA-SHA256("code=0|card=...|expire=...|maxwin=N")
// 验签失败说明是假服务器，返回 false。
func VerifyResp(code int, card string, expire int64, maxwin int, sigB64 string) bool {
	if sigB64 == "" {
		return false
	}
	pub, err := parsePublicKey(pubKeyPEM)
	if err != nil {
		fmt.Println("[cardkey] 解析内置公钥失败:", err)
		return false
	}
	plain := fmt.Sprintf("code=%d|card=%s|expire=%d|maxwin=%d", code, card, expire, maxwin)
	digest := sha256.Sum256([]byte(plain))
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return false
	}
	return rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], sig) == nil
}

// parsePublicKey 解析内置公钥 PEM（SubjectPublicKeyInfo 格式，BEGIN PUBLIC KEY）。
// 兼容 PKCS#1（BEGIN RSA PUBLIC KEY）作为后备，避免服务器误导出格式导致全部失败。
//
// 健壮性处理：JSON 配置文件里的 PEM 可能因复制粘贴带上不可见字符
// （BOM / 不间断空格 U+00A0 / 零宽字符 U+200B），或把换行多转义成字面 "\n"，
// 两种情况都会让 pem.Decode 精确匹配失败。这里按 原样 → 字面 \n 还原 → 剔除非打印字符 依次尝试。
func parsePublicKey(pemStr string) (*rsa.PublicKey, error) {
	pemStr = strings.TrimSpace(pemStr)
	if pemStr == "" {
		return nil, fmt.Errorf("公钥配置为空（license_pubkey 未填写）")
	}

	// 1) 原样解析
	block, _ := pem.Decode([]byte(pemStr))

	// 2) 字面 "\n" / "\r\n" 还原成真换行
	if block == nil {
		normalized := strings.ReplaceAll(pemStr, "\\r\\n", "\n")
		normalized = strings.ReplaceAll(normalized, "\\n", "\n")
		normalized = strings.ReplaceAll(normalized, "\\r", "\n")
		block, _ = pem.Decode([]byte(normalized))
	}

	// 3) 剔除所有非打印字符后再试（覆盖 BOM / 不可见字符）
	if block == nil {
		var sb strings.Builder
		for i := 0; i < len(pemStr); i++ {
			c := pemStr[i]
			if c == '\n' || c == '\r' || c == '\t' || (c >= 32 && c < 127) {
				sb.WriteByte(c)
			}
		}
		block, _ = pem.Decode([]byte(sb.String()))
	}

	if block == nil {
		return nil, fmt.Errorf("公钥 PEM 解析失败")
	}
	if pub, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if rsaPub, ok := pub.(*rsa.PublicKey); ok {
			return rsaPub, nil
		}
		return nil, fmt.Errorf("公钥不是 RSA")
	}
	if rsaPub, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return rsaPub, nil
	}
	return nil, fmt.Errorf("公钥格式无法解析（需 SubjectPublicKeyInfo）")
}

// ── AES-GCM 本地加密（.lic 标记） ──
// 密钥 = SHA256(设备指纹 + 固定盐)：换设备后密钥不同，解密失败自动重新激活

// deriveKey 由设备指纹派生 AES 密钥
func deriveKey(fp string) []byte {
	sum := sha256.Sum256([]byte(fp + "|FY-CARDSALT-2026"))
	return sum[:]
}

// aesGCMEncrypt 加密，输出格式: nonce(12) + ciphertext
func aesGCMEncrypt(plain, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plain, nil), nil
}

// aesGCMDecrypt 解密，输入格式: nonce(12) + ciphertext
func aesGCMDecrypt(data, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(data) < gcm.NonceSize() {
		return nil, fmt.Errorf("密文太短")
	}
	nonce := data[:gcm.NonceSize()]
	ct := data[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ct, nil)
}
