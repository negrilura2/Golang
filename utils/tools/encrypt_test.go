package tools

import (
	"testing"
)

var key = []byte("1234567890123456")
var key1 = []byte("1234567890321456")

func TestAESRoundTrip(t *testing.T) {
	plaintext := "hello"
	ares, err := AESEncrypt(plaintext, key)
	if err != nil {
		t.Fatalf("加密失败，错误为%v", err)
	}
	res, err := AESDecrypt(ares, key)
	if err != nil {
		t.Fatalf("解密失败，错误为%v", err)
	}
	if plaintext != string(res) {
		t.Errorf("解密出的明文和原明文不一致！")
	}
	t.Logf("解密出的明文%s,原明文%s", res, plaintext)
}

func TestAESDecryptWrongKey(t *testing.T) {
	plaintext := "hello"
	ares, err := AESEncrypt(plaintext, key)
	if err != nil {
		t.Fatalf("加密失败，错误为%v", err)
	}
	res, err := AESDecrypt(ares, key1)
	if err == nil {
		t.Fatalf("换错key应该失败但是没报错")
	}
	t.Logf("解密出的明文:%s,原明文:%s", res, plaintext)
}

func TestAESDecryptInvalidData(t *testing.T) {

	_, err := AESDecrypt("", key)
	if err == nil {
		t.Fatalf("空明文应该解密失败，却没有报错")
	}

	_, err = AESDecrypt("zzzz", key)
	if err == nil {
		t.Fatalf("非法hex密文应该报错，却没有报错")
	}

}
