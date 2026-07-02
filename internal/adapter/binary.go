package adapter

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
)

// BinaryResolver 负责查找外部二进制文件
// 查找优先级：customPath → resources/bin → 系统 PATH
type BinaryResolver struct {
	customPaths map[string]string // 用户配置的路径
	bundleDir   string            // 随包二进制目录
}

// NewBinaryResolver 创建 BinaryResolver
// customPaths: 用户在设置里配置的 name→path 映射
// bundleDir: 随包二进制目录（如 resources/bin）
func NewBinaryResolver(customPaths map[string]string, bundleDir string) *BinaryResolver {
	return &BinaryResolver{
		customPaths: customPaths,
		bundleDir:   bundleDir,
	}
}

// Resolve 查找指定名称的二进制文件
// name: 二进制名称（如 "ffmpeg"、"whisper-cli"）
func (r *BinaryResolver) Resolve(name string) (string, error) {
	// 1. 用户配置的路径
	if path, ok := r.customPaths[name]; ok && path != "" {
		if _, err := exec.LookPath(path); err == nil {
			return path, nil
		}
	}

	// 2. 随包目录
	if r.bundleDir != "" {
		exeName := name
		if runtime.GOOS == "windows" {
			exeName = name + ".exe"
		}
		bundlePath := filepath.Join(r.bundleDir, exeName)
		if _, err := exec.LookPath(bundlePath); err == nil {
			return bundlePath, nil
		}
	}

	// 3. 系统 PATH
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("binary %q not found in custom paths, bundle dir, or system PATH", name)
	}
	return path, nil
}
