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

|F37|沙盒任务已明确discarded，但任务记录和结果头部经通用stateLabel显示状态待确认|StudioJobs仅为合法discarded状态补已丢弃标签，保留其它状态和全部操作|实际沙盒桌面/手机截图定位；16项Studio测试通过，studio-sandbox/report-pass35.json四场景通过，采用/保存/发布/应用仍独立|
|F38|日程条目版本冲突复核以整段内部请求JSON为主体，手机难以核对将要重提的建议|SchedulePage按原冻结request显示活动、时间、地点、原因；完整请求与版本保留折叠，所有CAS/原key/命令保持|schedule-deep/pass39-item-conflict-mobile-conflict-review.png定位；新增实际React复核层级/原key断言，待本批运行和手机复拍|

验收工具校正：Diary fixture的replies原只按diary_id过滤，相同样本ID跨角色时混入其它fid留言，正式UI正确拒绝；fixture补fid过滤后reply/request与generation双视口复验通过。日记/媒体生成的部分旧失败是重复正文选择器或过快ready观察窗口，不是产品故障；合并后的report-consolidated保留最新逐案结果及原失败证据。

|F39|初始化服务已知awaiting_review时，参考图/设定图只显示等待任务与产物，入口仍为查看技术详情|creation-progress按已知session状态明确等待人工评审；仅未确认图片的对应行用前往审阅候选链接；PreparationSteps支持该行label，保留原href/returnTo与其它来源未知状态|preparation-read/report-before44.json桌面/手机复现；新增等待不计完成、已有可读图不回退的回归，待pass45实际复验|

|F40|保留工具Sheet/Dialog没有复刻theme，手机Sheet默认75%宽、输入16px/500；导入普通独立草稿固定红色确认；任务恢复自有Dialog层级/取消/焦点与其它页不同|OverlayHost/DisclosurePort真实portal根加纯replica-overlay主题，Sheet复用product-sheet并独立wide宽度，Dialog几何仅Dialog选择器；统一13px/400输入与可见焦点；WorkflowRecovery共用WorkspaceDialog保留form/footer关联及busy保护；durableCommandDrawer默认危险色不变，import仅显式false|admin-deep/report-pass49为修前；pass50/51实际18/20场景通过并测字体/宽度/焦点/主色；独立静态复核后补记录被拒绝审阅窗的实际入口，pass52两视口返回焦点通过。Web644全量、类型/构建52、相关DOM回归通过，无协议/Owner/原key变化|

|F41|暂停/继续/核对写入与主目标回读已成功，history API有新原因，但已打开详情的证据/历史仅在首次openGoal加载，仍显示旧记录|GoalsPage在已确认编辑/状态命令和原核对终态后，仅为当前所选目标重读两类记录；复用原scope/epoch/原游标重试，次级读取错误不翻转已确认结果，不重放mutation|feed-goals/report-before60.json两视口实测：authority有新历史但UI缺失；新增正常更新+次级GET失败独立重读React回归；pass63双视口通过，Goal13项及完整Web646项通过|

|F42|状态命令回执后只核对goal id，过旧主目标快照也会释放提交、显示已读回状态；与定义保存的版本校验不一致|GoalsPage状态onSaved要求安全整数revision不低于回执，同版本且回执为GoalStatus时核对status；reassess的pending不作GoalStatus比较，旧快照进入原回执只读恢复|新增React旧读回→GET恢复且pause仅一次；feed-goals/pass63两视口实际旧快照恢复通过；13项Goal与全量646通过|

跨日验收工具修正：2026-10-29零点后，Now测试用now-1h且未带local_date，样本落在前一天而被正确过滤。只把fixture改为当前本地日的明确全天计划，保留action_status=null的“不从时钟推断执行”断言；生产日期规则未变。pass62全量曾因该两测试失败，修正后pass63为646/646。Goal历史原因现按case唯一命名，避免其它场景的同名旧记录产生假通过。

|F43|创建页内从新草稿跳转resume历史参数时，URL变更但组件无key、长存控制器未重建，仍呈现新草稿，尽管原历史完整保留|App创建页使用path+query作为key，与现有Studio/Media路由身份约定一致；既有Owner隔离和creationRecovery保持|creation-supplement/report-pass65.json桌面同页恢复失败且history里保留精确createdId；pass66双视口同页resume与原激活只一次断言通过；14场景均通过，另pass64世界冲突两视口通过；类型/构建及Web646通过|

|F44|停止本次生成的POST失败后controller保留取消结果未知错误及running原消息，但ChatPage以!busy隐藏错误，通用映射还将明确说明变成泛化失败|ChatPage等待期间仍显示当前真实错误；chatFailureText保留控制器已知的取消结果未知说明；不改取消权限/请求/消息状态|chat-inbox-avatar/report-pass67.json两视口复现且仅1次原turn取消POST；新增实际React等待中取消失败→只读核对回归，浏览器复验待运行|

|F45|聊天头部角色选择框固定115×30，而Demo同样内容为90×39；局部覆盖误删了全局min-height并强制最大宽度|replica-chat既有局部规则恢复auto宽度与39px最小高度，保留90–115px范围、字号和原Select交互|shared-pass31同内容桌面配对定位；待最新双视口computed与配对图复验|

|F46|目标规划的恢复策略JSON无效时，仅显示通用“操作未完成，请稍后重试”，无法区分本地格式问题和服务失败|goal-planning的策略表单仅为本地SyntaxError给出明确JSON修正提示，保留其它错误映射、草稿和原change调用|retained-tools/pass71两视口定位；受控浏览器验证格式错误不再发送PUT且保留原输入，待最新构建复验|

|F47|诊断页嵌套“查询说明与维护”展开后为空，清理入口丢失；scanner仍处理父Disclosure移动后已脱离root的候选，随后移除这些临时port丢内容|DomainPrimitives扫描候选前重新核对root.contains，等原子树挂回再处理嵌套控件；保留原form/事件和通用组件|formal-edges/report-pass77两视口定位；浏览器DOM读到inner content为空，JSDOM无法复现但新增原表单/嵌套回调契约7项通过；实际修后待验|

|F48|侧栏角色切换使用Button默认500字重，而Demo Select400，126px同宽下角色职业提前省略；媒体导航误用Images多图标|workspace-replica原角色切换规则明确400；WorkspaceNavigation使用原Demo的Image单图标，保持Dropdown/键盘与切换操作|chat-pass71 computed确认同宽126px、400/500差异；待最终配对与导航复验|
|F49|聊天和此刻的装饰杯子/绿植受lucide版本影响，路径与原Demo不同，图片附件同样漂移|WorkspaceMomentArt固定原Demo实际SVG路径作为装饰资源；不更换普通操作图标库，不修改角色事实/图片URL|只读浏览器提取原Demo插画内两SVG，原参考不变；待同内容聊天/此刻配对复验|

|F50|日程日期Popover的Button被Radix改写data-slot后，未命中共享outline背景/边框/文字规则，正常状态出现黑色边框；同类组合按钮也遗漏主题|共享主题按Button原slot或明确size+variant标记匹配真实button/a，使用where保持低优先级，保留各业务类；不改交互和移动触控下限|shared-pass79配对中日程日期可见；待Calendar原交互与跨页几何复验|

|F51|人物卡较晚的语义Button规则将padding固定22覆盖700px下15，Badge固定10px也覆盖原8px；目标卡同样没有采用小屏Surface18px|移除人物Button冗余padding，按原断点恢复Badge8/3×5并把基础规则放在断点前；目标仅600px下18px|shared-pass79逐张配对定位，窄屏人物卡正文被额外挤窄换行，目标卡每行多8px；待双视口复拍|
|F52|管理二级导航窄屏flex-wrap形成三行、通知刷新独占一行，主体比Demo下移|管理900px下恢复单行横向滚动、按钮不收缩；通知700px下三列布局保留筛选与刷新，可用宽度内显示|shared-pass79两页窄屏对照定位；待滚动可达、筛选/刷新与几何复验|

|F53|日记手机收藏/搜索被强制新开整行；Studio刷新将新建动作挤到标题下，独立查询又新增一行，首张资源下移|日记保留桌面收藏checkbox，窄屏同一受控状态放入原搜索框内筛选菜单，双列对齐tabs/search；Studio原刷新与提交移入搜索框，原form和load仍保持，手机搜索跨全行|shared-pass79逐张复核定位，查询/收藏及原草稿未删除；待DOM和双视口交互/配对验证|

测试素材校正：媒体页Demo走MediaImage→Art→CharacterArt/GarmentArt（outfit固定outer/#b4b799），旧提取器只输出raw SVG，且outfit误取衣柜g-cardigan。新增媒体专用SVG/foreignObject由原MediaImage和原resources.css渲染，仅修测试资产与DTO URL；保留原头像/衣柜/通用portrait资源。人物裁切需实际修后截图确认，未改生产媒体图片或Demo参考。

2026-10-29本批验证结果：F44聊天取消结果未知两视口可见且只读核对；F45聊天选择框两视口90×39与Demo一致；F46非法规划JSON阻止额外PUT；F47诊断嵌套维护两视口恢复，完整导出/不完整流保护/原快照清理重试通过，另Admin18场景复验通过。F48字重400和媒体单图标已在final-pass85配对；F49装饰路径固定并复拍，媒体专用测试素材包装已记录；F50日期组合控件两视口白底/边框rgb(220,228,212)，Calendar关闭返回焦点与日期切换通过。F51人物15px/8px标签和目标18px移动padding实测；F52管理单行滚动末项可达、通知刷新同行通过；F53日记收藏菜单保持搜索草稿、原批注/收藏读回，Studio新建同行、9类型菜单、查询刷新不提交新草稿通过。responsive-layout/report-consolidated共14场景通过；56项相关DOM检查通过；最新全量Web649、类型87、构建86及生产守卫通过。

|F54|创作资源已发布徽标仍为无圆点灰色outline，字号/间距不符合原Demo状态层级|StudioDirectory保留实际row.status，额外以明确published_version和非archived标记呈现绿色已发布；恢复原9px/400、圆点与自然padding|仅展示标签；草稿/发布/应用仍独立；待目录与归档状态双视口复验|
|F55|所有通知固定Leaf，待处理各来源也固定ShieldCheck，无法从图标区分已有真实来源|Inbox按BrowserNotification.source_kind已支持的10种已提交事件映射图标；pending按正式5种group.kind映射；未知来源保留通用图标，不从示例ID/标题推断|不新增通知、不更改已读/处理/来源跳转；Demo的模拟task/persona通知无对应真实事件仍保留明确边界；待来源图标及已读/筛选/返回复验|

|F56|侧栏选中角色职业在狭窄区省略后没有完整查看提示，角色菜单也遗漏已有职业字段，偏离Demo的人物辨认信息|选中值保留完整title，菜单展示实际name/job并允许窄屏长词换行，保留ID选择和当前角色状态|不按ID补职业，不扩宽侧栏或删原文本；待角色作用域切换、窄屏关闭/焦点和菜单完整值复验|

|F57|记忆工具条搜索max480而Demo320，筛选包装160而Demo130，正常内容相同仍显著拉宽|按原lp-toolbar恢复320/190搜索范围和130筛选包装，选择值自然宽度/原padding，窄屏保留完整查询输入与两个筛选|final-pass85正常配对定位；只改局部CSS，不改输入、apply、cursor或读取|
|F58|通知标签保留默认Tabs line黑色下划线及42px高度，遗漏Demo SegTabs的绿线、间距、计数和底部留白；旧样式靠important抹默认样式|用原通知局部规则恢复SegTabs字重/绿线/间距/Badge，隐藏默认after；保留真实Radix tabs键盘及全部通知/未读状态，移除这些important|final-pass94逐张复核定位；待双视口未读切换、筛选/刷新与同行布局复验|

|F59|通知窄屏仍用40px图标、12/11px标题正文和10px时间；Demo断点为34px、11/10px及9px时间，造成消息密度与层级偏差|按原companion.css的700px断点恢复18×16内边距、11px gap、34px圆角图标与文字字号，时间色恢复原b0baa1|controls-pass98配对定位；不更改正文、待处理判断或通知操作，待最终双视口复拍与来源流程复验|

2026-10-29 F54–F59结果：status-presentation/pass95两视口4场景通过，覆盖发布/草稿/归档及10种正式通知来源、未来来源、constructor/__proto__等未知值；unknown值不借对象原型作图标，实际React回归保持正文和详情可用。Shell/pass95角色菜单7项断言×双视口通过，含完整title、真实职业和原scope切换；pass94空目录和焦点一并保留。响应式pass99两页×双视口通过，测得原绿线/34px图标/11-10-9px字级、记忆320px搜索，并验证筛选/刷新不覆盖新草稿、显式应用才切换；通知来源8场景复验通过。最新650/650 Web、0skip，类型95、构建99和生产守卫通过。pass95的唯一初次失败来自新增测试沿用前一测试的角色范围，已独立清除测试sessionStorage并在pass96全量通过；产品未调整筛选规则。

媒体测试素材最终证据：final-pass94保留同内容双视口完整原图/实现图/未遮罩diff；media-pass93额外量测同一原矢量在各自真实容器的绘制差异（包含参考图的收藏图标等允许差异），没有设置通过阈值。测试HTTP按实际img视口提供原MediaImage/Art矢量的固有尺寸，避免外部SVG默认300×150改变内联绘制；没有调整生产容器/图片逻辑或参考像素。截图外4张手机素材明确未量测。早期pass85空白素材、pass91越界/重复容器量测是验收工具问题，保留为历史，不作验收依据。

F57–F59最终确认：responsive-layout/pass99四个场景通过，包含通知原绿线、移动34px图标及11/10/9px文字、记忆320px搜索、实际类型筛选、保留新草稿的刷新与显式查询；chat-inbox-avatar/pass99八个通知来源/返回/失败/分类场景通过。final-pass99已人工查看两页手机配对，桌面及手机标题坐标一致。只完成本条已列范围，不代表所有深层状态均完成视觉验收。


## 2026-10-29 F60–F67：深层状态与原回执恢复

|ID|复现的问题|修复与边界|实际验证|
|---|---|---|---|
|F60|世界失败执行retry真实回执只有id/status/revision，UI却要求完整run，误报角色世界不匹配|仅最小回执分支按原run、queued、精确下一版校验；随后原GET仍校验完整scope|World 29项定向；实际浏览器14/14，含未知/GET失败/有效v0定义|
|F61|目标手动顺序混入候选/暂停ID，独立策略基线被候选审查静默推进|order只提交active；依赖仍保留全记录；候选/来源改变后显式核对策略基线|Goal规划12项定向；advanced-pass103两视口8/8|
|F62|目标策略已接受后GET失败仍可重写；未知回执不能恢复原请求；规划历史读取失败可能再请求|持久保留原body/key/CAS/原因与accepted检查点；已接受GET-only，未知重放原身份；规划受理与完成分开|定向覆盖重开与新草稿；浏览器readback/lost-response两视口通过|
|F63|目标旧编辑器迟到回执可能覆盖同Owner/fid的新输入，且读回缺少回执版次/字段约束|仅合并同key的ack；检查实际回执revision与投影，旧版或同版不一致保持恢复|Goal规划定向含late receipt、较旧与同版不一致、新草稿保留；全量通过|
|F64|真实checkbox变更同步重绘，blur又发change，重入DOM替换抛NotFoundError|共享立即change分类用于commit与blur；保留focusout、selection、FormData和文字blur提交|红测2处复现、原生控件15/15；实际Goal浏览器8/8无异常|
|F65|自我认知采用明确核对的最新基线后取消，详情/历史仍显示旧值|采用已校验详情并刷新目录、修订、Owner复核，不仅改变命令草稿版次|Self定向红绿；adopt-refresh两视口通过|
|F66|自我认知手动刷新只读详情，已挂载修订/复核记录不刷新|仅self分支同步刷新次级reader；保持原memory/reflection行为|同版新增Owner复核后实际刷新可见，两视口通过|
|F67|自我forget/restore回执读回同版但status不一致时错误清除恢复|同版须与回执status匹配；不一致保留GET-only；独立review和较新权威版语义保留|Self最终22项定向；实际10/10，含完整v1→v4谱系|

Studio应用到运行世界、Self完整历史和World定义的fixture补齐属于验收真实性改进。Studio没有复现新的生产缺陷。Studio实际2/2通过；Self最新合并10/10。全量Web694/694、0skip，build104与生产守卫通过。上传旧测试25ms固定等待已换成有界等待实际UI状态，仍检查同一上传身份、SHA、一次POST与重开恢复；没有MediaUpload生产修改。


|F68|首次所需目录读取失败被无session分支误显示成登录；成功重读未清旧bootstrap错误；失败卡被flex拉成整屏|优先呈现启动失败及原重读入口；成功仅清bootstrap错误，其他同Owner错误保留；提示卡self-start消除多余空白|新2项bootstrap红绿，定向5/5；实际目录失败/恢复双视口通过。Auth/Library合并16/16|
|F69|非法完整JSON修正并明确采用/撤销后仍显示“JSON尚未修正”|私有追踪本次parser错误；仅其仍拥有显示错误时清除，不清后来独立请求/读取错误|新增Studio红绿；定向24/24，七类型双视口14/14且每例断言纠正后无旧alert|

补验结果（pass108–116）：Studio运行拒绝/unknown/pending 6/6；账户/Library16/16；Profile空名/busy/实际基线冲突4/4；七类Studio14/14；换装failed/rejected/no-change/unknown/GET失败10/10。旧脚本等待失配保留：Studio恢复后应走“查看实际记录”，重开新应用会清旧回执；Profile成功后弹窗已关闭；Wear原确认未知后须先取消本地提示再点卡片原重试，成功态没有错误态的“查看原请求”按钮。没有为这些harness假设修改产品。

全量pass115 **697/697、0skip**；typecheck114、build116及生产守卫通过。build114已真实五页只读通过，build116只增加启动错误卡自身对齐，待最新静态同步。剩余六组确定补验清单见verification/remaining-state-backlog.md。


## F70–F78：全站反馈治理增量（2026-10-29）

F70为六类Feedback、保留错误默认与诊断/生命周期；F71为创建记录分类与精确历史/原请求；F72为聊天存储getter和运行中归档保护；F73为配置普通/角色原提交发现、全入口拦截、stale基线及存储重读；F74为字段校验、纠正清除与手机滚动；F75为普通结果去重/到期、上传代次隔离；F76为任务受理进度/缺runtime保护和保留工具实际错误重试；F77为此刻读取范围及端口显式语义；F78为手机弹窗/档案标签/人物详情键值布局。逐问题的原触发、文件拥有者、实际动作、恢复清除、测试和桌面/390证据见feedback-issues.md；精确源位置见evidence/feedback-source-locations.json。

全量717及后续配置28定向、typecheck33/build18通过；新受控浏览器172条、深层配对10组与人物复拍，按实际断言记录，不能视作177行全部通过。真实Web静态更新后五页实际API只读通过；没有后端或LLM策略变更。任务仍in_progress。

F79：深层反馈继续发现的同事件重复确认与聊天固定红色覆盖。命令组件压掉重复普通确认，ActionFeedback将真实失败原因和读回故障合在同一区域，保留两个真实动作和原输入披露；取消等信息按显式kind着色。全量718、类型35/构建21及深层People10/Feed8/ShellChatInbox12/Wear10通过。详见feedback-issues.md FG18。


## r19 空库回归增量

逐项原问题、文件、动作和恢复清除、三视口证据见[r19-acceptance.md](r19-acceptance.md)及[r19-issues.json](r19-issues.json)。本批34条含修复、语义确认、能力差异和未验范围，不是34个独立bug。详细检查索引evidence/r19-browser-index.json；不以受控HTTP冒充真实Core。
