# Checklist

- [x] `PickFile` 方法正确实现，通过 `application.Get().Dialog.OpenFile()` 弹出文件选择框，支持传入标题和文件过滤器
- [x] `PickDirectory` 方法正确实现，通过 `application.Get().Dialog.OpenFile()` 设置 `CanChooseDirectories(true).CanChooseFiles(false)` 弹出目录选择框
- [x] `ListProjects` 后端方法正确扫描项目目录并返回项目列表，按更新时间倒序
- [x] 前端 `bindings.d.ts` 声明了 `ListProjects`、`PickFile`、`PickDirectory` 的类型
- [x] 前端 `client.ts` 实现了 `listProjects`、`pickFile`、`pickDirectory` 函数
- [x] 首页 `Home.tsx` 正确展示项目卡片列表，卡片包含项目名称、状态、时长、更新日期
- [x] 无项目时首页显示空状态引导提示
- [x] 点击项目卡片可导航到对应工作台
- [x] 路由 `/` 正确指向 `Home` 组件
- [x] 新建项目页面视频文件路径旁有"选择文件"按钮
- [x] 点击"选择文件"弹出系统文件选择器，选择后路径自动填入并触发探测
- [x] 设置页面每个二进制路径输入框旁有"浏览"按钮
- [x] 设置页面模型目录输入框旁有"浏览"按钮
- [x] 点击"浏览"按钮弹出系统文件/目录选择器，选择后路径自动填入
- [x] 侧边栏导航新增"首页"入口
- [x] 构建无报错，`go build` 和 `npm run build` 均通过