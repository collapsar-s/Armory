// 工具类型识别：由路径前缀/后缀推断 type（app | jar | py | sh | bin | url）。
// .command/.zsh 与后端 DetectType 保持一致，都归为 shell 脚本。
export function detectType(path) {
  const p = (path || '').trim().toLowerCase()
  if (p.startsWith('http://') || p.startsWith('https://')) return 'url'
  if (p.endsWith('.jar')) return 'jar'
  if (p.endsWith('.py')) return 'py'
  if (p.endsWith('.sh') || p.endsWith('.command') || p.endsWith('.zsh')) return 'sh'
  if (p.endsWith('.app')) return 'app'
  return 'bin'
}
