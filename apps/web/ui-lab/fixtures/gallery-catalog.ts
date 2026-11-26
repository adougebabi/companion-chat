// Single source for the gallery's components and their applicable states.
// Shared by the UI Lab page and e2e/gallery.spec.ts (plain data, no imports).
export const GALLERY_CATALOG = {
  'mobile-navigation': { title: '手机主导航', overlay: false, states: ['normal', 'long', 'disabled'] },
  'bottom-sheet': { title: '手机底部操作', overlay: true, states: ['normal', 'long', 'empty', 'loading', 'error'] },
  'workspace-dialog': { title: '业务详情与编辑浮层', overlay: true, states: ['page', 'sheet', 'dialog', 'long', 'busy'] },
  'compact-dialog': { title: '手机简短确认', overlay: true, states: ['normal', 'long', 'busy'] },
  page: { title: '页面容器与内容区', overlay: false, states: ['normal', 'long', 'empty', 'loading', 'error'] },
  'detail-sheet': { title: '详情抽屉', overlay: true, states: ['normal', 'long', 'empty', 'loading', 'error'] },
  'edit-sheet': { title: '编辑抽屉', overlay: true, states: ['normal', 'long', 'disabled', 'busy', 'error'] },
  tabs: { title: '响应式标签导航', overlay: false, states: ['normal', 'long', 'disabled'] },
  form: { title: '表单分组与底部操作区', overlay: false, states: ['normal', 'long', 'disabled', 'error'] },
  'task-status': { title: '异步任务状态', overlay: false, states: ['queued', 'running', 'succeeded', 'failed', 'cancelled', 'long'] },
  feedback: { title: '信息 / 待处理 / 警告 / 错误反馈', overlay: false, states: ['information', 'action-required', 'warning', 'error', 'long'] },
  'record-list': { title: '列表与空数据', overlay: false, states: ['normal', 'long', 'empty', 'loading', 'error', 'stale'] },
  'detail-fields': { title: '详情字段', overlay: false, states: ['normal', 'long', 'empty'] },
  'settings-form': { title: '新建设置表单（仅组合共享组件）', overlay: true, states: ['normal', 'busy', 'error'] },
  // Existing product components migrated onto ResponsiveSheet (real code, fixture data).
  'migrated-detail-drawer': { title: '已迁移：DetailDrawer', overlay: true, states: ['normal', 'long'] },
  'migrated-drawer-frame': { title: '已迁移：DrawerFrame', overlay: true, states: ['normal', 'long'] },
} as const;
export type GalleryId = keyof typeof GALLERY_CATALOG;
