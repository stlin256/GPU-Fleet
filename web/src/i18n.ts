import { createContext, useContext } from 'react';
import { AppLanguage } from './api';

export type Translate = (key: string, values?: Record<string, string | number>) => string;

export const languages: Array<{ code: AppLanguage; label: string; nativeLabel: string }> = [
  { code: 'zh-CN', label: 'Chinese', nativeLabel: '简体中文' },
  { code: 'en-US', label: 'English', nativeLabel: 'English' }
];

const en: Record<string, string> = {
  '正在连接': 'Connecting',
  '检查当前 Web 会话': 'Checking the current web session',
  '未选择': 'Not selected',
  '端口范围应为 1-65535': 'Port must be between 1 and 65535',
  '密码至少 8 位': 'Password must be at least 8 characters',
  '新密码至少 8 位': 'New password must be at least 8 characters',
  '两次密码不一致': 'Passwords do not match',
  '证书和私钥需要同时上传': 'Certificate and private key must be uploaded together',
  '配置已保存，重启服务后端口或 HTTPS 生效': 'Configuration saved. Restart the service for port or HTTPS changes to take effect.',
  '配置已保存，必要时重启服务后生效': 'Configuration saved. Restart the service if required.',
  '配置已保存': 'Configuration saved',
  '首次配置': 'First-time setup',
  '配置引导': 'Setup wizard',
  '初始化服务访问参数': 'Initialize service access settings',
  '访问密码': 'Access password',
  '新密码': 'New password',
  '至少 8 位': 'At least 8 characters',
  '留空则不变': 'Leave blank to keep unchanged',
  '确认密码': 'Confirm password',
  '再次输入密码': 'Enter the password again',
  '仅修改密码时填写': 'Required only when changing the password',
  '访问端口': 'Access port',
  '界面语言': 'Interface language',
  'HTTPS 证书': 'HTTPS certificate',
  '私钥文件': 'Private key file',
  '取消': 'Cancel',
  '保存中': 'Saving',
  '保存配置': 'Save configuration',
  '当前证书到期：{date}': 'Current certificate expires: {date}',
  '登录面板': 'Dashboard login',
  '登录后记住当前设备 30 天': 'This device is remembered for 30 days after login',
  '密码': 'Password',
  '登录中': 'Logging in',
  '登录': 'Log in',
  'GPU 资源总览': 'GPU Resource Overview',
  '设备管理': 'Device Management',
  'GPU 监控': 'GPU Monitoring',
  '服务设置': 'Service Settings',
  '总览': 'Overview',
  '设备': 'Devices',
  '设置': 'Settings',
  '服务端时间 {time}': 'Server time {time}',
  '等待服务端数据': 'Waiting for server data',
  '刷新': 'Refresh',
  '退出登录': 'Log out',
  '磁盘空间低于保护阈值，服务端已拒绝新指标写入。': 'Disk space is below the protection threshold. New metric writes are rejected.',
  '切换浅色': 'Switch to light mode',
  '切换深色': 'Switch to dark mode',
  '多机 GPU 运行态': 'Multi-host GPU Runtime',
  '{devices} 台设备，{gpus} 块 GPU，按最新上报状态汇总。': '{devices} devices, {gpus} GPUs, summarized from the latest reports.',
  '等待客户端上报 GPU 运行信息。': 'Waiting for clients to report GPU runtime data.',
  '在线设备': 'Online devices',
  'GPU 总数': 'Total GPUs',
  '忙碌 GPU': 'Busy GPUs',
  '高温 GPU': 'Hot GPUs',
  '总显存用量': 'Total memory usage',
  '总功耗': 'Total power',
  'GPU 数量': 'GPU count',
  '平均利用率': 'Average utilization',
  'GPU 详细状态': 'GPU Details',
  '卡片化查看多设备 GPU 运行状态': 'Card view for multi-device GPU runtime',
  '暂无 GPU 上报': 'No GPU reports yet',
  '离线': 'Offline',
  'GPU 利用率': 'GPU utilization',
  '最近 1 小时': 'Last 1 hour',
  '显存': 'Memory',
  '总量 {value}': 'Total {value}',
  '温度': 'Temperature',
  '功耗': 'Power',
  '上限 {value}': 'Limit {value}',
  '利用率分布': 'Utilization Distribution',
  '当前快照': 'Current snapshot',
  '峰值 {value}': 'Peak {value}',
  '空闲 GPU': 'Idle GPUs',
  '活跃 GPU': 'Active GPUs',
  '{label} 历史趋势图': '{label} history trend chart',
  '过热': 'Overheated',
  '偏高': 'Elevated',
  '正常': 'Normal',
  '刚刚': 'Just now',
  '{value}s 前': '{value}s ago',
  '{value}m 前': '{value}m ago',
  '{value}h 前': '{value}h ago',
  '采集异常': 'Collection error',
  '高温': 'Hot',
  '关注': 'Watch',
  '显存空闲': 'Memory free',
  '显存保留': 'Memory reserved',
  '显存利用': 'Memory utilization',
  '温度上限': 'Temperature limit',
  '显存温度': 'Memory temperature',
  '功耗上限': 'Power limit',
  '风扇': 'Fan',
  '图形时钟': 'Graphics clock',
  '显存时钟': 'Memory clock',
  'SM 时钟': 'SM clock',
  '视频时钟': 'Video clock',
  'PCIe 当前': 'PCIe current',
  'PCIe 最大': 'PCIe max',
  '显示': 'Display',
  '驱动模型': 'Driver model',
  '新设备密钥': 'New device secret',
  '已创建设备 {name}': 'Created device {name}',
  '设备已启用': 'Device enabled',
  '设备已禁用': 'Device disabled',
  '已轮换密钥': 'Secret rotated',
  '设备密钥已轮换': 'Device secret rotated',
  '设备已删除': 'Device deleted',
  '设备名称已更新为 {name}': 'Device name updated to {name}',
  '注册设备': 'Register Device',
  '设备别名': 'Device alias',
  '创建中': 'Creating',
  '创建': 'Create',
  '设备列表': 'Device List',
  '设备名称': 'Device name',
  '保存': 'Save',
  '修改设备名称': 'Rename device',
  '改名': 'Rename',
  '禁用设备': 'Disable device',
  '启用设备': 'Enable device',
  '禁用': 'Disable',
  '启用': 'Enable',
  '轮换密钥': 'Rotate secret',
  '轮换': 'Rotate',
  '删除设备': 'Delete device',
  '删除': 'Delete',
  '暂无设备': 'No devices',
  '目标设备': 'Target device',
  '处理中': 'Processing',
  '允许 {name} 使用现有密钥继续上报 GPU 指标。': 'Allow {name} to continue reporting GPU metrics with the existing secret.',
  '确认启用': 'Confirm enable',
  '禁用后 {name} 的上报请求会被服务端拒绝，客户端本机配置不会被修改。': 'After disabling, reports from {name} will be rejected. Local client configuration is not changed.',
  '确认禁用': 'Confirm disable',
  '旧密钥会立即失效，需要在 {name} 所在机器手动更新新密钥后才能继续上报。': 'The old secret expires immediately. Update the new secret on {name} before it can report again.',
  '确认轮换': 'Confirm rotate',
  '删除后 {name} 将从设备列表和最新 GPU 快照中移除，原 Agent 密钥会失效。': 'After deletion, {name} is removed from the device list and latest GPU snapshots. The old Agent secret expires.',
  '确认删除': 'Confirm delete',
  '复制密钥': 'Copy secret',
  '已复制': 'Copied',
  '复制': 'Copy',
  'GPU 进程': 'GPU Processes',
  '暂无 GPU 进程快照': 'No GPU process snapshots',
  '24 小时统计': '24-hour Stats',
  '服务状态': 'Service Status',
  '等待服务端配置': 'Waiting for server configuration',
  '需要重启': 'Restart required',
  '当前协议': 'Current protocol',
  '证书已配置': 'Certificate configured',
  '未启用证书': 'Certificate not enabled',
  '证书到期': 'Certificate expiry',
  '未配置': 'Not configured',
  'HTTPS 下次启动生效': 'HTTPS takes effect on next start',
  'HTTPS 已启用': 'HTTPS enabled',
  'HTTP 模式': 'HTTP mode',
  '磁盘预留': 'Disk reserve',
  '空闲 {value}': '{value} free',
  '访问与安全': 'Access & Security',
  '凭据、端口、语言和 HTTPS 证书': 'Credentials, port, language, and HTTPS certificate',
  '重新打开端口、密码、语言和证书配置流程': 'Reopen the port, password, language, and certificate setup flow',
  '打开引导': 'Open wizard',
  '维护与发布': 'Maintenance & Release',
  '数据库、在线更新和版本信息': 'Database, online update, and release information',
  '更新已构建完成，服务端正在自动重启': 'Update built. Server is restarting automatically',
  '，预计 {date} 前后恢复': ', expected to recover around {date}',
  '更新已拉取并构建完成，正在等待服务端重启': 'Update pulled and built. Waiting for server restart.',
  '恢复后页面会自动刷新。': 'The page will refresh automatically after recovery.',
  '版本已更新': 'Version updated',
  '服务端已自动重启并刷新页面。': 'The server restarted automatically and the page has been refreshed.',
  'HTTPS 证书已启用': 'HTTPS certificate enabled',
  'HTTPS 证书已保存，服务端已自动重启并刷新页面。': 'HTTPS certificate saved. The server restarted automatically and the page has been refreshed.',
  '知道了': 'OK',
  '当前已经是最新版本': 'Already up to date',
  '在线更新': 'Online Update',
  '未绑定上游': 'No upstream',
  '检查 Git 上游版本': 'Check Git upstream version',
  '当前提交': 'Current commit',
  '远端提交': 'Remote commit',
  '落后': 'Behind',
  '超前': 'Ahead',
  '远端': 'Remote',
  '检查时间': 'Checked at',
  '检查中': 'Checking',
  '检查更新': 'Check update',
  '更新中': 'Updating',
  '拉取并重启': 'Pull and restart',
  '更新代理': 'Update proxy',
  '保存代理': 'Save proxy',
  '更新代理已保存': 'Update proxy saved',
  '更新代理已清空': 'Update proxy cleared',
  '已发送更新请求': 'Update request sent',
  '依赖预检、构建远端提交并执行 fast-forward 拉取': 'Checking dependencies, building the remote commit, and fast-forward pulling',
  '更新已应用，准备自动重启': 'Update applied. Preparing automatic restart.',
  '服务端正在自动重启': 'Server is restarting automatically',
  '等待服务端恢复，恢复后自动刷新': 'Waiting for the server to recover. The page will refresh automatically.',
  '正在读取 Git 状态': 'Reading Git status',
  '失败': 'Failed',
  '未知': 'Unknown',
  '尚未读取更新状态': 'Update status has not been read',
  '不可用': 'Unavailable',
  '服务端未运行在 Git 工作区': 'Server is not running in a Git working tree',
  '已阻止': 'Blocked',
  '服务端工作区存在未提交改动，已阻止自动拉取': 'Server working tree has uncommitted changes. Automatic pull is blocked.',
  '未绑定': 'Unbound',
  '当前分支没有 Git upstream': 'Current branch has no Git upstream',
  '分叉': 'Diverged',
  '本地和上游存在分叉，不能自动 fast-forward': 'Local and upstream have diverged. Fast-forward is unavailable.',
  '本地超前': 'Local ahead',
  '本地提交超前上游，面板不会执行拉取': 'Local commits are ahead of upstream. The dashboard will not pull.',
  '有新版本': 'Update available',
  '{count} 个提交可拉取、构建并自动重启': '{count} commits can be pulled, built, and restarted automatically',
  '最新': 'Latest',
  '已经是最新版本': 'Already up to date',
  '加载中': 'Loading',
  '版本与变更': 'Version & Changes',
  'GPUFleet 发布信息': 'GPUFleet release information',
  '作者': 'Author',
  '版本': 'Version',
  '提交': 'Commit',
  '构建时间': 'Build time',
  '仓库地址': 'Repository',
  '最近变更': 'Latest changes',
  '正在读取版本信息': 'Reading release information',
  '打开 GitHub': 'Open GitHub',
  '新增': 'Added',
  '变更': 'Changed',
  '安全': 'Security',
  '修复': 'Fixed',
  '密码更改': 'Password Change',
  '仅使用密码作为 Web 凭据': 'Password is the only web credential',
  '当前密码': 'Current password',
  '密码已更新': 'Password updated',
  '更新密码': 'Update password',
  '端口配置': 'Port Configuration',
  '当前监听端口': 'Current listening port',
  '端口已保存，重启后生效': 'Port saved. Restart to apply.',
  '端口已保存': 'Port saved',
  '保存端口': 'Save port',
  '语言设置': 'Language Settings',
  '控制首次配置、面板和后续设置页语言': 'Controls first-time setup, dashboard, and settings language',
  '语言已保存': 'Language saved',
  '保存语言': 'Save language',
  '证书已保存，重启后启用 HTTPS': 'Certificate saved. Restart to enable HTTPS.',
  '证书已保存，服务端正在自动重启。恢复后页面会自动刷新。': 'Certificate saved. The server is restarting automatically. The page will refresh after recovery.',
  '证书已保存': 'Certificate saved',
  '到期 {date}': 'Expires {date}',
  '证书文件': 'Certificate file',
  '选择文件': 'Choose file',
  '未选择文件': 'No file selected',
  '上传中': 'Uploading',
  '上传证书': 'Upload certificate',
  '数据库下载': 'Database Download',
  '数据库大小': 'Database size',
  '下载数据库': 'Download database',
  '请求过于频繁，请等待 {duration} 后再试': 'Too many requests. Retry after {duration}.',
  'language endpoint not found; rebuild and restart the server binary': 'Language endpoint not found; rebuild and restart the server binary.'
};

const enPatterns: Array<[RegExp, string]> = [
  [/^(\d+) 台设备，(\d+) 块 GPU，按最新上报状态汇总。$/, '$1 devices, $2 GPUs, summarized from the latest reports.'],
  [/^服务端时间 (.+)$/, 'Server time $1'],
  [/^总量 (.+)$/, 'Total $1'],
  [/^上限 (.+)$/, 'Limit $1'],
  [/^峰值 (.+)$/, 'Peak $1'],
  [/^数据库大小 (.+)$/, 'Database size $1'],
  [/^空闲 (.+)$/, '$1 free'],
  [/^当前证书到期：(.+)$/, 'Current certificate expires: $1'],
  [/^到期 (.+)$/, 'Expires $1'],
  [/^已创建设备 (.+)$/, 'Created device $1'],
  [/^设备名称已更新为 (.+)$/, 'Device name updated to $1'],
  [/^允许 (.+) 使用现有密钥继续上报 GPU 指标。$/, 'Allow $1 to continue reporting GPU metrics with the existing secret.'],
  [/^禁用后 (.+) 的上报请求会被服务端拒绝，客户端本机配置不会被修改。$/, 'After disabling, reports from $1 will be rejected. Local client configuration is not changed.'],
  [/^旧密钥会立即失效，需要在 (.+) 所在机器手动更新新密钥后才能继续上报。$/, 'The old secret expires immediately. Update the new secret on $1 before it can report again.'],
  [/^删除后 (.+) 将从设备列表和最新 GPU 快照中移除，原 Agent 密钥会失效。$/, 'After deletion, $1 is removed from the device list and latest GPU snapshots. The old Agent secret expires.'],
  [/^(\d+) 个提交可拉取、构建并自动重启$/, '$1 commits can be pulled, built, and restarted automatically'],
  [/^更新已构建完成，服务端正在自动重启，预计 (.+) 前后恢复$/, 'Update built. Server is restarting automatically, expected to recover around $1'],
  [/^更新已构建完成，服务端正在自动重启，预计 (.+) 前后恢复。恢复后页面会自动刷新。$/, 'Update built. Server is restarting automatically, expected to recover around $1. The page will refresh automatically after recovery.'],
  [/^(\d+) 天$/, '$1 days'],
  [/^(\d+) 天 (\d+) 小时$/, '$1 days $2 hours'],
  [/^(\d+) 小时$/, '$1 hours'],
  [/^(.+) 历史趋势图$/, '$1 history trend chart']
];

const textSources = new WeakMap<Text, string>();
const attrSources = new WeakMap<Element, Record<string, string>>();
let observer: MutationObserver | undefined;

export function makeTranslator(language: AppLanguage): Translate {
  return (key, values) => {
    const template = language === 'en-US' ? en[key] ?? key : key;
    if (!values) return template;
    return Object.entries(values).reduce(
      (text, [name, value]) => text.split(`{${name}}`).join(String(value)),
      template
    );
  };
}

export function translateText(value: string, language: AppLanguage): string {
  if (language === 'zh-CN') return value;
  const trimmed = value.trim();
  if (!trimmed) return value;
  const exact = en[trimmed];
  if (exact) return value.replace(trimmed, exact);
  for (const [pattern, replacement] of enPatterns) {
    if (pattern.test(trimmed)) return value.replace(trimmed, trimmed.replace(pattern, replacement));
  }
  return value;
}

export function installDOMI18n(getLanguage: () => AppLanguage) {
  const apply = () => translateRoot(document.body, getLanguage());
  observer?.disconnect();
  observer = new MutationObserver(() => apply());
  apply();
  observer.observe(document.body, {
    childList: true,
    subtree: true,
    characterData: true,
    attributes: true,
    attributeFilter: ['title', 'placeholder', 'aria-label']
  });
  return () => observer?.disconnect();
}

export function translateRoot(root: ParentNode, language: AppLanguage) {
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  const textNodes: Text[] = [];
  while (walker.nextNode()) textNodes.push(walker.currentNode as Text);
  for (const node of textNodes) {
    if (!node.parentElement || node.parentElement.closest('script,style,code')) continue;
    const stored = textSources.get(node);
    const current = node.nodeValue ?? '';
    let source = stored ?? current;
    if (stored && language !== 'zh-CN') {
      const translatedStored = translateText(stored, language);
      if (current !== stored && current !== translatedStored) source = current;
    }
    if (!textSources.has(node) || source !== stored) textSources.set(node, source);
    const next = translateText(source, language);
    if (node.nodeValue !== next) node.nodeValue = next;
  }
  const elements = root instanceof Element ? [root, ...Array.from(root.querySelectorAll('*'))] : Array.from(root.querySelectorAll('*'));
  for (const element of elements) {
    if (element.closest('script,style,code')) continue;
    const stored = attrSources.get(element) ?? {};
    let changed = false;
    for (const attr of ['title', 'placeholder', 'aria-label']) {
      const current = element.getAttribute(attr);
      if (!current) continue;
      let source = stored[attr] ?? current;
      if (stored[attr] && language !== 'zh-CN') {
        const translatedStored = translateText(stored[attr], language);
        if (current !== stored[attr] && current !== translatedStored) source = current;
      }
      stored[attr] = source;
      const next = translateText(source, language);
      if (current !== next) element.setAttribute(attr, next);
      changed = true;
    }
    if (changed) attrSources.set(element, stored);
  }
}

export type I18nContextValue = {
  language: AppLanguage;
  setLanguage: (language: AppLanguage) => void;
  t: Translate;
};

export const I18nContext = createContext<I18nContextValue>({
  language: 'zh-CN',
  setLanguage: () => undefined,
  t: makeTranslator('zh-CN')
});

export function useI18n() {
  return useContext(I18nContext);
}
