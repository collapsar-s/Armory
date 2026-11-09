// 分类层级选项（共享）：供「添加工具 / 编辑工具」的所属分类下拉使用。
//
// 规则：
//   - 返回按主分组归组的数据，配合 <optgroup> 渲染：主分组名作组头，
//     分组（子级）用短名缩进展示在组内——避免「漏洞利用 / Spring 系」这类
//     全路径把下拉拉得又长又重复（主分组一多时不可读）；
//   - 每组第一项是该主分组本身（工具可以直接挂主分组）。
export function buildCatOptions(categories) {
  const out = []
  for (const c of categories) {
    if (c.parentId) continue
    out.push({
      id: c.id,
      name: c.name,
      children: categories.filter((x) => x.parentId === c.id),
    })
  }
  return out
}
