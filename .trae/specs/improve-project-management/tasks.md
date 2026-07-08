# Tasks

- [x] Task 1: 后端新增文件选择与项目列表接口
  - [x] 1.1 在 `app/app.go` 中新增 `PickFile` 方法，通过 `application.Get().Dialog.OpenFile()` 弹出文件选择框，支持传入标题和文件过滤器，返回选中路径
  - [x] 1.2 在 `app/app.go` 中新增 `PickDirectory` 方法，通过 `application.Get().Dialog.OpenFile()` 设置 `CanChooseDirectories(true).CanChooseFiles(false)` 弹出目录选择框，返回选中路径
  - [x] 1.3 在 `internal/service/project.go` 中新增 `ListProjects` 方法，扫描 `projectsDir` 下所有子目录中的 `project.json`，读取并反序列化，按更新时间倒序返回
  - [x] 1.4 在 `app/app.go` 中新增 `ListProjects` 方法，调用 `projectService.ListProjects()`

- [x] Task 2: 前端 API 层更新
  - [x] 2.1 在 `frontend/src/api/bindings.d.ts` 中声明 `ListProjects`、`PickFile`、`PickDirectory` 的 TS 类型
  - [x] 2.2 在 `frontend/src/api/client.ts` 中实现 `listProjects`、`pickFile`、`pickDirectory` 函数

- [x] Task 3: 创建项目列表首页
  - [x] 3.1 新建 `frontend/src/pages/Home.tsx`，实现项目卡片式列表页面，包含空状态提示和项目卡片网格布局
  - [x] 3.2 项目卡片组件显示：项目名称、状态标签、视频时长、更新日期，可点击跳转到工作台
  - [x] 3.3 在 `frontend/src/App.tsx` 中更新路由：`/` 路由指向 `Home` 组件，`/project/new` 保持不变

- [x] Task 4: 创建项目页面集成文件选择框
  - [x] 4.1 修改 `frontend/src/pages/NewProject.tsx`，在视频文件路径输入框旁添加"选择文件"按钮
  - [x] 4.2 点击"选择文件"按钮调用 `pickFile` API，弹出系统文件选择器，选择后自动填入路径并触发探测

- [x] Task 5: 设置页面集成文件选择器
  - [x] 5.1 修改 `frontend/src/pages/Settings.tsx`，为每个二进制路径输入框添加"浏览"按钮，调用 `pickFile` 选择可执行文件
  - [x] 5.2 为模型目录输入框添加"浏览"按钮，调用 `pickDirectory` 选择目录

- [x] Task 6: 更新侧边栏导航
  - [x] 6.1 修改 `frontend/src/layouts/AppLayout.tsx`，添加"首页"导航项，图标使用 `Home`（lucide-react）

# Task Dependencies
- Task 2 依赖 Task 1（前端 API 需要后端方法）
- Task 3、4、5、6 均依赖 Task 2（前端页面需要 API 层）
- Task 3、4、5、6 可并行开发