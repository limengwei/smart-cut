package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// resolveWhisperModel 将用户配置的模型目录解析为具体的 .bin 模型文件路径。
// 约定：在目录下查找 *.bin（排除常见非模型 bin），优先 base/small/medium/large，取首个匹配。
// 若传入的就是 .bin 文件则直接返回。
func resolveWhisperModel(modelDirOrFile string) (string, error) {
	info, err := os.Stat(modelDirOrFile)
	if err != nil {
		return "", fmt.Errorf("模型路径不可访问: %w", err)
	}

	// 如果直接是文件，直接用
	if !info.IsDir() {
		if strings.ToLower(filepath.Ext(modelDirOrFile)) != ".bin" {
			return "", fmt.Errorf("模型文件不是 .bin: %s", modelDirOrFile)
		}
		return modelDirOrFile, nil
	}

	entries, err := os.ReadDir(modelDirOrFile)
	if err != nil {
		return "", fmt.Errorf("读取模型目录失败: %w", err)
	}

	priority := []string{"base", "small", "medium", "large", "tiny", "q5", "q4"}

	var bins []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.ToLower(e.Name())
		if !strings.HasSuffix(name, ".bin") {
			continue
		}
		// 排除明显不是模型的 bin（如自定义脚本等，这里宽松处理，只按后缀）
		bins = append(bins, e.Name())
	}

	if len(bins) == 0 {
		return "", fmt.Errorf("模型目录 %s 下未找到任何 .bin 模型文件", modelDirOrFile)
	}

	sort.Slice(bins, func(i, j int) bool {
		ri, rj := modelRank(bins[i], priority), modelRank(bins[j], priority)
		if ri != rj {
			return ri < rj
		}
		return bins[i] < bins[j]
	})

	return filepath.Join(modelDirOrFile, bins[0]), nil
}

// modelRank 返回模型名在优先级表中的排名（越小越优先），未命中返回 len(priority)
func modelRank(name string, priority []string) int {
	lower := strings.ToLower(name)
	for i, p := range priority {
		if strings.Contains(lower, p) {
			return i
		}
	}
	return len(priority)
}
