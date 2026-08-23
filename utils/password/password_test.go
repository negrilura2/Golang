package password

import (
	"errors"
	"golang.org/x/crypto/bcrypt"
	"testing"
)

func TestHashPassword(t *testing.T) {
	hash1, err := HashPassword("123456")
	if err != nil {
		t.Fatalf("HashPassword失败: %v", err)
	}
	hash2, err := HashPassword("123456")
	if err != nil {
		t.Fatalf("HashPassword失败: %v", err)
	}
	t.Logf("hash1:%s hash2:%s", hash1, hash2)
}

func TestVerifyPassword(t *testing.T) {
	hash1, err1 := HashPassword("123456")
	if err1 != nil {
		t.Fatalf("HashPassword失败:%v:", err1)
	}
	err := VerifyPassword(hash1, "123456")
	if err != nil {
		t.Error("正确密码应该通过")
	}
	err = VerifyPassword(hash1, "wrongness")
	if !errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		t.Errorf("错误密码应该返回 ErrMismatchedHashAndPassword，实际: %v", err)
	}

	hash2, err2 := HashPassword("123456")
	if err2 != nil {
		t.Fatalf("HashPassword失败:%v", err2)
	}
	if hash1 == hash2 {
		t.Error("有盐，两次哈希应该不同")
	}
}

func TestErrorTypes(t *testing.T) {
	hash1, err1 := HashPassword("123456")
	if err1 != nil {
		t.Fatalf("HashPassword失败：%v:", err1)
	}
	errWrong := VerifyPassword(hash1, "1234544") //应该是ErrMismatchHashAndPassword
	if !errors.Is(errWrong, bcrypt.ErrMismatchedHashAndPassword) {
		t.Errorf("错误异常，应该是密码错误的异常,而现在是:%v", errWrong)
	}
	errWrong = VerifyPassword("123456", "123456")
	if errors.Is(errWrong, bcrypt.ErrMismatchedHashAndPassword) {
		t.Errorf("错误异常，应该是hash太短的错误，而不是:%v", errWrong)
	}
}
