package app

import (
	"os/exec"
	"strings"

	"smart-cut/internal/model"
)

func (a *App) GetSettings() (*model.GlobalSettings, error) {
	settings, err := a.configManager.Load()
	if err != nil {
		return nil, NewAppError(ErrCodeInternal, "加载设置失败", err.Error())
	}
	return settings, nil
}

func (a *App) SaveSettings(s model.GlobalSettings) error {
	if err := a.configManager.Save(&s); err != nil {
		return NewAppError(ErrCodeInternal, "保存设置失败", err.Error())
	}
	return nil
}

func (a *App) ProbeBinary(name string) (string, string, error) {
	path, err := a.binaryResolver.Resolve(name)
	if err != nil {
		return "", "", NewAppError(ErrCodeEnv, "未找到二进制文件 "+name, err.Error())
	}

	version := probeBinaryVersion(name, path)
	return path, version, nil
}

func probeBinaryVersion(name, path string) string {
	var cmd *exec.Cmd
	switch name {
	case "ffmpeg", "ffprobe":
		cmd = exec.Command(path, "-version")
	case "whisper-cli":
		cmd = exec.Command(path, "--help")
	default:
		cmd = exec.Command(path, "--version")
	}

	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	lines := strings.Split(string(out), "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return ""
}