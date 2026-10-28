# 实际修复清单（持续更新）

|ID|现象与根因|修改位置|当前证据与边界|
|---|---|---|---|
|F01|聊天头部有实际头像，消息列表只给name/initial而未传src|ChatPage.tsx消息CharacterAvatar传同一_directory.avatar|源码独立复核；真实头像API/私有字节已验；实际浏览器图片加载待验|
|F02|缺图时姓名末字与Demo首字不同|CharacterAvatar.tsx、workspace.mjs统一首个Unicode码点|源码复核；浏览器正常/缺图/切角色待验|
|F03|say跨页请求未消费；已有草稿缺少显式合并/放下|App传route；ChatPage作用域检查与promptOffer|demo-fullsite-shared真实组件回归通过：旧稿保留、显式加入/放下、错误fid拒绝、空白稿预填、无自动发送；实际Owner转场/浏览器待验|
|F04|快速前往只取侧栏页，漏掉创建入口|WorkspaceQuickJump补“新的相识/初始化角色”|demo-replica-library键盘/搜索导航回归通过；浏览器待验|
|F05|角色库创作入口丢失character_template筛选|LibraryPage正式链接附kind参数|组件断言通过；真实导航待验|
|F06|目标回执同版本只核对标题/动机，其他提交字段失配也报成功|GoalsPage onSaved核对criteria/deadline/policy/protection、创建scope及显式目标，保留新版本处理|受理后四类字段错读均留在只读恢复，恢复仅GET，1次mutation；专项通过。未声称已发现真实Core丢字段|
|F07|日程建议卡期待从未冻结的before字段，并遗漏请求已有的结束时间/地点/原因|SchedulePage准确说明旧快照缺失，完整显示提案区间/场景/原因；缺activity不冒称取消|真实组件建议展示回归通过；不伪造原安排，不改Core请求|
|F08|真实Goal stages/commitments/dependencies仅藏在完整JSON，缺少可读分组|GoalsPage按实际字段呈现阶段/推进条件/承诺窗口/阻碍，保留各记录完整来源|结构化真实DTO组件回归通过，不创建新Owner写API，不据阶段推断总目标完成；视觉待验|

所有待验项继续保留，未以本清单代替完整覆盖矩阵或浏览器对照。

|ID|现象与根因|修改位置|当前证据与边界|
|---|---|---|---|
|F09|公共导航逐行漂移/内容少15px；Sidebar li margin不折叠和scrollbar-gutter stable|workspace-replica.css修原布局规则，统一导航行高和margin，移除常驻空白gutter|shared-pass2/normal-pass4桌面/窄屏标题与主框宽度核对；不是像素阈值自动通过|
|F10|角色库窄屏横幅高13.5px、标签过高；装饰SVG包装参与flex、强制min-height|Library StoryMark wrapper contents；tag原规则行高/字号/自然高度|shared-pass2同视口主标题坐标一致；真实搜索/列表/详情返回通过|
|F11|当前活动当成下一项，今天摘要丢掉过去安排；目录丢失真实最近会话时间/颜色|nextScheduleItem；Now当天末4条；contract-map保留时间与颜色；Library真实相对时间|shared tests/Now相关测试通过，截图保留真实未提供天气，不伪造Demo天气|
|F12|通知角标只数第一批30条，冷启动没读count；角色筛选仅本地；标全部不更新共享计数|notification-counter.mjs/session generation+epoch；workspace+App+Inbox调用全局count，服务端fid筛选，写后后台刷新|counter/approved-workspace/inbox14项通过；实际inbox-read-sync通过（测试HTTP）；不把已读当处理完成|
|F13|聊天图片在气泡外、方形144px，缺少图片失败恢复|ChatPage BubbleContent内Attachment+ChatAttachment225×150/caption，WorkspaceDialog；原URL重读|生产构建实际浏览器打开/ESC/焦点、图片失败重读通过；独立检查指出重读失焦已修同节点，最后复验中|
|F14|Button默认32px、disabled65%、outline边框和字色偏深；Portal主题不完整|共享Button按Demo变体36px/50%，保留全部API和group/button；workspace共享控件样式覆盖Portal|typecheck/build通过；normal-pass4/shared-pass5含桌面和窄屏；空白/纯空格发送禁用、ShiftEnter实际浏览器通过|
|F15|Card新registry ring叠出第二圈；已完成状态被全局.done划线|workspace低权重Card单边框规则；react-layout .done限制到.goal.done|同内容Studio/Now/Schedule对照，保留实际执行状态|
|F16|World/People搜索图标遗漏；Studio摘要clamp样式没在其作用域定义；Profile引用错误文案|补搜索图标原结构与缩进；Studio原r-clamp-2定义；Profile优先真实identity.tagline，不造fallback tagline|世界/人物/创作详情实际浏览器通过，完整内容仍可详情读取；假默认性格维度未复制进生产|
|F17|动态头像只取末字，忽略真实角色头像；列表时间带全绝对说明挤占元信息|FeedPost复用CharacterAvatar实际目录；Feed/Media compactDateTime并保留title/dateTime|同内容截图；动态评论/媒体详情浏览器通过；日期缺失不造日期|
|F18|通知/管理样式仍在components layer，被其余复刻页无layer控件压过|原replica-management.css移除layer包装，保持原规则顺序/hidden强制保护；已读按钮恢复小型元操作|独立只读检查无隐藏/selected/disabled明确回归；management-pass7桌面窄屏已复拍，二级全量仍待验|
|F19|手机导航Escape后焦点落到页面；非角色页目录职业缺失，footer过高触发导航自动滚动|Sidebar.returnFocus+App源ref；contract-map/workspace保留目录职业；footer固定29px按钮与单行MY WORKSPACE|真实手机Escape/切页/无溢出通过；目录职业投影回归已补；footer跨页复拍待最后核对|

|F20|管理中心重复28px容器间距、目录按钮w-fit和缺少服务分区标题层级|管理根gap0、目录全宽、正式Tabs恢复Demo线段样式；服务图标/状态小字与标题说明恢复，保留controller与绑定行为|来自正常对照的具体差异，修复后待复拍与设置二级浏览器检查|

|F21|初始化预览性格滑块因profileFields无personality而全部禁用；手机主题标签换行；身份字段顺序偏离Demo|Wizard按完整预览更新性格值（保留分析权威和其它结构）；身份按Demo排序、未知字段仍保留末尾；标签单行横向滚动；结构正文按原标签/内容列排版|由同内容creation-preview五主题双视口截图定位；键盘改值→完整JSON→刷新保留回归正在运行|

|F22|角色作用域媒体子页重复选择角色，手机操作区多一行；非媒体库页的顶栏刷新实际只刷新隐藏媒体库|MediaWorkspace仅全局非媒体库显示目标选择；媒体库刷新留在媒体库，各子页用自身重读；恢复生成描述示例、头像职业/独立保存层级；共享Badge按Demo字号/圆角|visual-normal三子页双视口截图定位；角色作用域/全局目标选择与现有媒体回归待本批复验|
