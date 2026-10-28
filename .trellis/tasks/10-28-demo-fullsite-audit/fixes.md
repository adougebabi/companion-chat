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

|F23|目标证据/历史首读缺失items时，undefined进入records并在渲染时.map致整页空白|GoalsPage在采用记录前确认items及对象条目；保留原详情/原读取重试。parity同时修复记录端点误返回Goal行|由实际目标详情崩溃定位；新增畸形读取→原详情保留→重读恢复回归，待运行|
|F24|地点编辑完整WorldForm只占外层lp-form一列，内部再分两列造成字段仅四分之一宽；弹窗遮罩过浅|WorldForm使用既有lp-wide跨全列，原r-form-stack规则同时匹配同节点的页面类以恢复20px间距；DialogOverlay按Demo颜色/4px模糊|由dialogs/world-place-edit双视口对照定位，修复后待复拍|
|F25|资料表单值从外层Label继承500字重；选中文字仍使用浏览器蓝色而非Demo浅绿|共享输入/选择值恢复400字重，复制原Demo选中文字背景；标签字重与交互保持|由profile/world/wardrobe弹窗实际配对图定位，待本批复拍|

|F26|动态发布/分享、人物新建/编辑/接触/应用整组表单仅占一列，部分根容器未匹配堆叠间距；实测桌面251.5/521、手机138.5/295宽|完整组用已有lp-wide；Feed根标记其样式作用域；Profile堆叠规则补同节点根匹配|form-groups/report-before.json记录实际宽度与display:block，修复后正在双视口复验|

补充核查：媒体动作已有.replica-media .lp-form单列保护，后补lp-wide不改变显示；已撤掉冗余类并纠正F24范围。媒体评审完整宽度回归仍保留。

|F27|当前穿着分隔符偏离Demo；原始验收场景穿着标记/文字不一致妨碍比较|WearingSummary只把确认条目用原Demo中点分隔；测试快照统一原有四件，参考仅经原生方案换装同步|source/reference-scene.mjs显式记录前置操作；生产穿着仍完全来自真实worn_items，待新一轮同内容截图|

|F28|衣物示意图虽有明确颜色描述仍全部用默认绿灰，活跃聊天状态文案偏离Demo|矢量示意按已有明确色词选用原Demo色调，无说明保持中性，显式color prop仍优先；正常聊天状态用原文案“生活进行中”|仅展示，不写回颜色/不新增Core字段/不依赖示例ID；待正常衣柜双视口对照|

|F29|当前表达画像整组结构与正文混在主卡，存在其他人格记录时不能直接当当前正文|仅突出API已有portrait_text，沿用原语义行解读并保留段落/状态/完整记录；profile_id与当前runtime不匹配时明确待核对，不借用其他人格正文|正常fixture用原Demo当前人格描述/语气作测试画像，技术结构按真实既有DTO核对；生产不从人设文案补运行画像。新增当前/不同profile回归待运行|

|F30|手机档案仍沿用桌面23px gap；原始Badge标签被局部统一浅绿覆盖，当前人格标签固定20px；性格条颜色偏离原Demo生效全局规则|replica-profile恢复18px断点、标签各自variant和自然高度、人格状态的点/间距、正文行高；indicator采用原Demo index.css实际#5d8766|normal-pass23/shared-pass26配对；after-pass23/26 computed。初次代理仅查primary得到#295442，主线程核对生效全局规则后纠正为#5d8766。原始before-pass22检测未等动画且重叠构建，只为探索，不当验收证据|
|F31|新Card自带text-sm导致默认20px行框，原Demo普通Surface继承21px；时间线p受旧全局1.8行高影响；附件caption继承错误行高|workspace作用域Card/base恢复1.5，时间线p明确1.5、聊天meta明确1.5/caption1.95；衣柜确认穿着标签按原Status自然高度和颜色|原有实际宽高差异逐项记录；最终共享复拍待新build后运行。保持原数据和图像URL，不固定卡高|
|F32|记忆纠正回执未知，已读取并明确采用最新基线后取消，详情和目录仍显示旧正文/版本|MemoryPage.memoryCommand.adoptLatest同步已校验current到详情、来源版本并刷新列表/历史；保留当前输入，不调用纠正mutation|浏览器memory-deep/report-pass24.json记录修前两视口失败；report-pass26.json的12场景全部通过（含此路径仅1次PUT、保存后GET失败只读恢复），DOM12/12通过|
|F33|记忆编辑的整段说明作为lp-form直接子项只占半列，窄屏产生过高细长说明块|replica-life的lp-form直接replica-info跨全列，保留其它字段分组|实际unknown手机图定位，添加实际note/form宽度断言；待最终复拍|
|F34|同一Studio编辑页发布后，版本列表读取identity仅resource.id，导致历史未包含刚发布版本|StudioLifecycle版本读取identity加入真实published_version，旧不可变版本仍可查|studio-deep/report-before-pass28.json桌面世界/角色模板实测失败；补已有真实React发布测试，待本批复验|

|F35|900px以下标题操作区缺少Demo的align-start规则；目标页额外刷新按钮挤宽顶栏，390px新增一整行；记忆卡继承Button500字重|恢复原900px断点；目标刷新移入搜索框保留原两组GET/disabled/label，查询清除独立定位；记忆卡400正文|shared-pass31同内容图已亲看：目标操作区回到第一行，记忆卡正文恢复；11项Goal检查通过，独立静态复核无明确问题|
|F36|手机日记目录摘要从网格子项溢出，原有ellipsis因justify-items:start下子项未被约束而失效|仅为目录摘要span恢复100%宽度与min-width:0，继续保留完整正文，不截断业务数据|shared-pass29/31实际截图定位；待最终窄屏宽度断言/复拍|

最新核验：F23的畸形记录回归、F29的12项Profile检查、F32的12项Memory检查、F34的16项Studio检查均通过；Web全量pass30为642/642、0skip，随后仅F35布局/操作区搬移再跑Goal11项。F24–F26八个完整表单双视口几何通过；F27/F28正常穿着同内容、色词示意图已复拍。最终几何after-pass31中档案hero桌面/手机宽高与Y、手机此刻timeline卡片尺寸/Y相同；桌面Now仍2px内栏宽差、手机档案下方正式披露/字段内容增加、衣柜行高等差异继续记录，不宣称全站视觉通过。
