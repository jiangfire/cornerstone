package db

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolveBootstrapPassword_EnvWins(t *testing.T) {
	t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "from-env-1234")
	pw, source, err := resolveBootstrapPassword()
	if err != nil {
		t.Fatalf("resolveBootstrapPassword 失败: %v", err)
	}
	if pw != "from-env-1234" {
		t.Fatalf("env 优先级未生效，得到 %q", pw)
	}
	if !strings.Contains(source, "BOOTSTRAP_ADMIN_PASSWORD") {
		t.Fatalf("source 标签应包含 BOOTSTRAP_ADMIN_PASSWORD，实际 %q", source)
	}
}

func TestResolveBootstrapPassword_RejectsShortEnv(t *testing.T) {
	t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "short")
	if _, _, err := resolveBootstrapPassword(); err == nil {
		t.Fatal("过短的 BOOTSTRAP_ADMIN_PASSWORD 应被拒绝")
	}
}

func TestResolveBootstrapPassword_RandomFallback(t *testing.T) {
	t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "")
	pw1, source, err := resolveBootstrapPassword()
	if err != nil {
		t.Fatalf("resolveBootstrapPassword 失败: %v", err)
	}
	if len(pw1) < 12 {
		t.Fatalf("随机密码长度过短: %d", len(pw1))
	}
	if source != "crypto/rand" {
		t.Fatalf("source 应为 crypto/rand，实际 %q", source)
	}

	pw2, _, err := resolveBootstrapPassword()
	if err != nil {
		t.Fatalf("第二次调用失败: %v", err)
	}
	if pw1 == pw2 {
		t.Fatal("两次随机生成应不相同")
	}
}

func TestWriteBootstrapCredentialsFile_WritesAndProtects(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BOOTSTRAP_ADMIN_FILE_DIR", dir)

	path, err := writeBootstrapCredentialsFile("admin", "S3cret-pass-#1")
	if err != nil {
		t.Fatalf("writeBootstrapCredentialsFile 失败: %v", err)
	}
	if filepath.Dir(path) != dir {
		t.Fatalf("路径未落到自定义目录: %s", path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取凭据文件失败: %v", err)
	}
	if !strings.Contains(string(content), "S3cret-pass-#1") {
		t.Fatal("凭据文件应包含密码")
	}
	if !strings.Contains(string(content), "username: admin") {
		t.Fatal("凭据文件应包含用户名")
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat 失败: %v", err)
		}
		mode := info.Mode().Perm()
		if mode != 0o600 {
			t.Fatalf("凭据文件权限应为 0600，实际 %o", mode)
		}
	}
}

func TestWriteBootstrapCredentialsFile_DoesNotOverwriteExisting(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BOOTSTRAP_ADMIN_FILE_DIR", dir)

	first, err := writeBootstrapCredentialsFile("admin", "original-pass")
	if err != nil {
		t.Fatalf("第一次写入失败: %v", err)
	}

	second, err := writeBootstrapCredentialsFile("admin", "different-pass")
	if err != nil {
		t.Fatalf("第二次调用不应返回错误: %v", err)
	}
	if first != second {
		t.Fatalf("应返回同一路径")
	}

	content, _ := os.ReadFile(first)
	if !strings.Contains(string(content), "original-pass") {
		t.Fatal("已存在的凭据文件不应被新密码覆盖")
	}
	if strings.Contains(string(content), "different-pass") {
		t.Fatal("文件不应包含新密码（不能覆盖）")
	}
}

func TestGenerateRandomPassword_NoLessThan12(t *testing.T) {
	pw, err := generateRandomPassword(8)
	if err != nil {
		t.Fatalf("generateRandomPassword 失败: %v", err)
	}
	if len(pw) < 12 {
		t.Fatalf("即使 length<12 也应至少 12 位，实际 %d", len(pw))
	}
}
