# 边界与设计

真实入口 main.tsx → App.tsx；诊断与安静时段经 pages/ManagementPage.jsx → views/ManagementPage.jsx，角色切换经 WorkspaceNavigation，日记和通知直接使用 pages，创建经 CharacterWizardPage → views/CharacterWizard，导入经 DomainPage → v3-domain/data-tools.js → DomainPrimitives → FieldInput → FilePicker。
只修这些入口的展示及必要 CSS，保留请求、身份、草稿、保存和恢复协议。复用现有 shadcn Badge、Field、Switch、DropdownMenu、Empty、Card、Button。
先对当前 production dist 记录基线，再针对仍存在的实际偏差编辑；已正确的项补真实入口测试，不伪造根因。
