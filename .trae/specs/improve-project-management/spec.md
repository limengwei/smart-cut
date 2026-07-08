# 项目管理完善 Spec

## Why
当前项目管理能力薄弱：仅支持在新建项目时手动输入文件路径（无系统文件选择框），首页直接跳转到新建页而无项目列表，设置页面的路径配置项也缺少系统文件选择器。需要完善项目管理流程，提升用户体验。

## What Changes
- 新增项目列表首页，以卡片式排列展示所有已创建的项目
- 创建项目页面支持通过系统文件选择框选择视频文件目录
- 设置页面：二进制路径和模型目录配置项支持通过系统文件选择器选择
- 后端新增 Go 方法：`ListProjects`、`PickFile`、`PickDirectory`（通过 Wails3 Dialog API）
- 前端新增首页路由和项目列表页面，调整路由结构
- **BREAKING**: 原首页路由 `/` 从跳转到 `/project/new` 变更为展示项目列表

## Impact
- Affected specs: 无（新增功能）
- Affected code: `app/app.go`, `internal/service/project.go`, `frontend/src/App.tsx`, `frontend/src/pages/NewProject.tsx`, `frontend/src/pages/Settings.tsx`, `frontend/src/layouts/AppLayout.tsx`, `frontend/src/api/client.ts`, `frontend/src/api/bindings.d.ts`, `frontend/src/stores/project.ts`

## ADDED Requirements

### Requirement: 项目列表首页
系统 SHALL 提供一个首页，以卡片式排列展示所有已创建的项目，每个卡片显示项目名称、状态、缩略图和最后更新时间。

#### Scenario: 首页展示项目列表
- **WHEN** 用户打开应用
- **THEN** 首页展示所有已创建项目的卡片式列表

#### Scenario: 无项目时显示空状态
- **WHEN** 用户尚未创建任何项目
- **THEN** 首页显示空状态提示，引导用户新建项目

#### Scenario: 点击项目卡片进入工作台
- **WHEN** 用户点击某个项目卡片
- **THEN** 系统导航到该项目的工作台页面

### Requirement: 系统文件选择框选择视频文件
创建项目时，系统 SHALL 提供"选择文件"按钮，通过 Wails3 `OpenFileDialog` 打开系统原生文件选择器，支持选择视频文件。

#### Scenario: 通过文件选择器选择视频
- **WHEN** 用户在新建项目页点击"选择文件"按钮
- **THEN** 系统弹出原生文件选择对话框，仅显示视频文件
- **WHEN** 用户选择文件后确认
- **THEN** 文件路径自动填入输入框，并触发文件探测

#### Scenario: 文件选择对话框取消
- **WHEN** 用户在文件选择对话框中点击取消
- **THEN** 路径输入框保持不变，无错误提示

### Requirement: 设置页面文件路径选择
设置页面中二进制路径和模型目录配置项 SHALL 提供"浏览"按钮，通过 Wails3 文件对话框打开系统原生选择器。

#### Scenario: 选择二进制文件路径
- **WHEN** 用户在设置页面点击二进制路径旁的"浏览"按钮
- **THEN** 系统弹出文件选择对话框，仅显示可执行文件
- **WHEN** 用户选择文件后确认
- **THEN** 路径自动填入对应输入框

#### Scenario: 选择模型目录
- **WHEN** 用户在设置页面点击模型目录旁的"浏览"按钮
- **THEN** 系统弹出目录选择对话框
- **WHEN** 用户选择目录后确认
- **THEN** 目录路径自动填入输入框

### Requirement: 后端项目列表接口
后端 SHALL 提供 `ListProjects` 方法，扫描项目存储目录，返回所有已创建的 `Project` 列表。

#### Scenario: 列出所有项目
- **WHEN** 前端调用 `ListProjects`
- **THEN** 返回所有已保存项目的数组（按更新时间倒序）

### Requirement: 后端文件选择接口
后端 SHALL 通过 Wails3 `application.Get().Dialog` 提供 `PickFile` 和 `PickDirectory` 方法供前端调用。

#### Scenario: 选择单个文件
- **WHEN** 前端调用 `PickFile` 并传入标题和文件过滤器
- **THEN** 弹出系统文件选择框，返回选中文件的完整路径

#### Scenario: 选择目录
- **WHEN** 前端调用 `PickDirectory` 并传入标题
- **THEN** 弹出系统目录选择框，返回选中目录的完整路径