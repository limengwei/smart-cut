package adapter

import (
	"os/exec"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// systemCommandName 返回一个系统一定存在的命令名
func systemCommandName() string {
	if runtime.GOOS == "windows" {
		return "cmd"
	}
	return "sh"
}

func TestBinaryResolver_CustomPath(t *testing.T) {
	// 用系统自带的命令模拟：先从 PATH 找到它，再作为 customPath 传入
	name := systemCommandName()
	path, err := exec.LookPath(name)
	require.NoError(t, err)

	resolver := NewBinaryResolver(map[string]string{"test-bin": path}, "")

	resolved, err := resolver.Resolve("test-bin")
	require.NoError(t, err)
	assert.Equal(t, path, resolved)
}

func TestBinaryResolver_SystemPATH(t *testing.T) {
	// 不提供 customPath 和 bundleDir，走系统 PATH
	resolver := NewBinaryResolver(nil, "")

	path, err := resolver.Resolve(systemCommandName())
	require.NoError(t, err)
	assert.NotEmpty(t, path)
}

func TestBinaryResolver_NotFound(t *testing.T) {
	resolver := NewBinaryResolver(nil, "")

	_, err := resolver.Resolve("this-binary-does-not-exist-12345")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
