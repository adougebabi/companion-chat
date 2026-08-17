# 知觉 Companion Chat

面向 LM Studio 与 ComfyUI 的本地多人格 AI 聊天服务。项目不运行或复制任何模型，所有推理仍由已启动的 LM Studio 和 ComfyUI 进程承担。

## 运行

```bash
npm install
npm start
```

默认打开 `http://localhost:4178`。服务地址、数据目录、LM Studio 与 ComfyUI 地址均可由环境变量配置，见 [`.env.example`](.env.example)。设置页面中的地址会覆盖首次启动时的环境变量默认值。

## Docker / NAS 部署

```bash
cp .env.example .env
# 修改 .env 中的 LM_STUDIO_URL 与 COMFYUI_URL
docker compose up -d --build
```

应用数据会保存在 `companion-data` Docker volume 中的 SQLite 数据库。首次启动会自动迁移已有的 `data/state.json`，但不会删除原文件；确认迁移完成后请自行备份该旧文件。

容器中的 `localhost` 指向容器自身：LM Studio 位于局域网其他机器时填写该机器的局域网地址；ComfyUI 也在 Compose 网络中时使用服务名，例如 `http://comfyui:8188`。

GitHub 推送到 `main` 或推送 `v*` 标签时，[`docker-publish.yml`](.github/workflows/docker-publish.yml) 会构建并发布 `companion-chat` 镜像。仓库需要设置两个 Actions Secrets：`DOCKERHUB_USERNAME` 和 `DOCKERHUB_TOKEN`。

## 长期记忆与人格演化

每个人格有一份初始人格 `initialPrompt` 和一份会演化的有效基础人格 `basePrompt`。用户发送消息后不会立即触发额外推理；当该人格连续空闲 10 分钟后，后台演化器才会成为候选任务。服务每 5 分钟扫描一次候选状态，并只对“自上一次演化后有新对话”的人格执行一次审阅。

审阅器会同时阅读初始人格、当前有效基础人格、近期 40 条对话与当前长期记忆，输出一份新的有效基础人格和新的长期记忆。每次变更会记录前后版本、原因和时间，保留最近 30 次历史。初始人格仍作为角色、专业边界和安全底线，用户无法通过一句指令将其替换。

冲突规则是：**用户最新且更具体的明确表达 > 旧的用户偏好 > 基础人格默认倾向**。例如，健身教练的基础人格是“偏好高强度训练”，用户说“强度太高”后会得到“训练强度：偏低或渐进”的记忆；它不会修改基础提示词，而是作为更高优先级的用户偏好持续生效。记忆面板会显示当前有效项及来源时间。

## AI 自主的生图与生视频

在设置中从 ComfyUI 使用 `File -> Export Workflow (API)` 导出工作流 JSON。将正向提示词节点的文本写为 `{{prompt}}`，粘贴到对应工作流配置。

用户不需要输入任何指令。人格在需要展示动作、器材、镜头或视觉方案时，会自主向系统请求相应的图片或视频工作流。例如，用户说“我今天想练腿”，健身教练可以先给出组数、次数和理由，再自然地依次附上动作 A、动作 B 的示范图。

生成任务由 Node 服务端提交、轮询并写回会话。关闭或刷新浏览器不会取消任务；重新打开时会从同一份会话数据恢复。服务端一次只处理一个生成任务，以便为 LM Studio 与 ComfyUI 保留资源。

## 资源策略

- 服务使用单文件 SQLite，没有额外数据库守护进程、向量库或后台 embedding 任务。
- 只有用户发消息后才请求 LM Studio；人格演化仅在 10 分钟空闲后执行，且每轮对话最多执行一次。
- ComfyUI 任务只有队列中存在任务时才进行低频轮询；空闲时不会请求 ComfyUI。
- 生产启动不含 Vite、热更新或文件监听。

`data/` 是运行时数据，包含 SQLite 数据库、完整本地聊天上下文和调试追踪，已被 `.gitignore` 排除。不要将它提交到 Git 或公开共享。
