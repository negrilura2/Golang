package tools

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

func Sha256Hash(text string) string {
	// 创建一个新的sha256哈希对象
	hash := sha256.New()

	// 将字符串写入哈希对象
	hash.Write([]byte(text))

	// 从哈希对象中获取哈希值
	hashBytes := hash.Sum(nil)

	// 将字节切片转换为十六进制字符串
	return hex.EncodeToString(hashBytes)
}

// AESEncrypt 加密函数
func AESEncrypt(plaintext string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	blockSize := block.BlockSize()
	origData := PKCS7Padding([]byte(plaintext), blockSize)
	iv := make([]byte, blockSize)
	blockMode := cipher.NewCBCEncrypter(block, iv)

	encrypted := make([]byte, len(origData))
	blockMode.CryptBlocks(encrypted, origData)

	return fmt.Sprintf("%x", encrypted), nil
}

// AESDecrypt 解密函数
func AESDecrypt(cryptotext string, key []byte) ([]byte, error) {
	cryptoBytes, err := hex.DecodeString(cryptotext)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	blockSize := block.BlockSize()
	iv := make([]byte, blockSize)
	blockMode := cipher.NewCBCDecrypter(block, iv)

	origData := make([]byte, len(cryptoBytes))
	blockMode.CryptBlocks(origData, cryptoBytes)

	return PKCS7UnPadding(origData)
}

// PKCS7Padding 填充函数
func PKCS7Padding(src []byte, blockSize int) []byte {
	padding := blockSize - len(src)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(src, padText...)
}

// PKCS7UnPadding 去除填充函数
func PKCS7UnPadding(src []byte) ([]byte, error) {
	length := len(src)
	if length == 0 {
		return nil, errors.New("填充字符串为空")
	}
	unpadding := int(src[length-1])
	if unpadding < 1 || unpadding > aes.BlockSize || unpadding > length {
		return nil, errors.New("unpadding不符合要求")
	}
	for i := len(src) - unpadding; i < len(src); i++ {
		if src[i] != byte(unpadding) {
			return nil, errors.New("unpadding不匹配")
		}
	}
	return src[:(length - unpadding)], nil
}
