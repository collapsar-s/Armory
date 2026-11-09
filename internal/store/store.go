// Package store 实现 Armory 的持久化层：负责把工具/分类/设置读写到
// ~/.armory/tools.json，并管理图标缓存目录 ~/.armory/icons。
//
// 设计要点：
//   - 用 sync.RWMutex 做读写分离，纯读路径共享读锁；
//   - 所有落盘采用「先写 .tmp 再 rename」的原子写，避免半截文件；
//   - 数据字段与 JSON tag 与历史版本完全一致（见 internal/model），
//     因此能直接读取旧版 ~/.armory/tools.json。
package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"armory/internal/model"
)

// Store 是持久化的核心对象。所有对数据的访问都必须经由它的方法。
type Store struct {
	mu   sync.RWMutex
	path string
	data model.RegistryData
}

// overrideDir 测试专用：非空时 DataDir/IconDir 返回该目录，避免测试污染真实用户数据。
var overrideDir string

// SetDataDirForTest 仅供单元测试注入临时数据目录；传空串恢复默认 ~/.armory。
func SetDataDirForTest(dir string) { overrideDir = dir }

// New 打开（或初始化）~/.armory/tools.json 并返回 Store。
// 文件不存在时会写入一份带默认分类与设置的数据文件；
// 文件存在但解析失败会返回错误（不静默覆盖用户数据）。
func New() (*Store, error) {
	s := &Store{path: filepath.Join(DataDir(), "tools.json")}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

// DataDir 返回数据目录 ~/.armory（测试期间为注入目录），并确保其存在。
func DataDir() string {
	if overrideDir != "" {
		_ = os.MkdirAll(overrideDir, 0o755)
		return overrideDir
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		// 极端情况下无法取到 home，退化到当前目录，避免 panic。
		home = "."
	}
	d := filepath.Join(home, ".armory")
	_ = os.MkdirAll(d, 0o700) // 仅当前用户可进入（数据文件含 0600，目录与文件权限配套）
	return d
}

// IconDir 返回图标缓存目录 ~/.armory/cache/icons，并确保其存在（与原版一致）。
func IconDir() string {
	d := filepath.Join(DataDir(), "cache", "icons")
	_ = os.MkdirAll(d, 0o755)
	return d
}

// IconPathFor 返回指定工具图标的缓存路径。
func IconPathFor(id string) string {
	return filepath.Join(IconDir(), id+".png")
}

// NewID 生成一个新的唯一 ID（16 位十六进制字符串）。
func NewID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand 失败极少见，用时间戳兜底保证唯一性可用。
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

// load 读取数据文件；不存在时初始化默认数据并落盘。
func (s *Store) load() error {
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.data = model.RegistryData{
				Tools:      []model.Tool{},
				Categories: DefaultCategories(),
				Settings:   model.Settings{Theme: "auto", Hotkey: "alt+space"},
			}
			return s.saveLocked()
		}
		return fmt.Errorf("读取工具库失败: %w", err)
	}
	if err := json.Unmarshal(b, &s.data); err != nil {
		return fmt.Errorf("tools.json 损坏: %w", err)
	}
	// 兼容：旧数据可能没有分类，补上默认分类。
	if len(s.data.Categories) == 0 {
		s.data.Categories = DefaultCategories()
	}
	if s.data.Tools == nil {
		s.data.Tools = []model.Tool{}
	}
	// 概念统一（2026-11-08）：「标题」类型并入分组，读入时归一 Kind，落盘后旧值消失。
	for i := range s.data.Categories {
		if s.data.Categories[i].Kind == "title" {
			s.data.Categories[i].Kind = ""
		}
	}
	return nil
}

// saveLocked 将数据原子写入磁盘（调用方必须已持有写锁）。
// 权限 0600：工具清单属于敏感数据（渗透作业的软件资产列表），仅限当前用户读写。
func (s *Store) saveLocked() error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化数据失败: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("写入临时文件失败: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("替换数据文件失败: %w", err)
	}
	return nil
}

// DefaultCategories 返回首次启动时的默认分类。
// ID 方案与原版一致："cat-" + 名称；目录不写 Kind（空值即视作 dir，与原版 JSON 输出一致）。
func DefaultCategories() []model.Category {
	names := []string{"信息收集", "漏洞利用", "代理隧道", "开发日常"}
	out := make([]model.Category, 0, len(names))
	for i, n := range names {
		out = append(out, model.Category{ID: "cat-" + n, Name: n, Order: float64(i)})
	}
	return out
}

// ---------- 读路径 ----------

// ListTools 返回指定分类下的工具；categoryID 为空或 "all" 时返回全部。
// 与原版一致：查看某个“父目录/标题”时，需返回其**整棵子树**（含子目录/标题）内的
// 工具——前端 listTools 后会按 categoryId 再把结果分区到各子区，若只返回直属工具，
// 子目录分区将永远为空。返回的副本保证调用方不会意外修改内部数据。
func (s *Store) ListTools(categoryID string) []model.Tool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var allowed map[string]bool
	if categoryID != "" && categoryID != "all" {
		allowed = s.subtreeIDsLocked(categoryID)
	}
	out := make([]model.Tool, 0, len(s.data.Tools))
	for _, t := range s.data.Tools {
		if allowed != nil && !allowed[t.CategoryID] {
			continue
		}
		t.HasIcon = s.iconExistsLocked(t.ID)
		out = append(out, t)
	}
	// 按 Order 升序稳定排序：未排序（0）的工具保持收录顺序（与原版一致）。
	sort.SliceStable(out, func(i, j int) bool { return out[i].Order < out[j].Order })
	return out
}

// ReorderCategories 按给定 id 顺序重排分类（侧栏主分组 / 页内分组的拖拽排序）。
// 只更新出现过的 id 的 Order（按索引），未提及的分类保持原值。
func (s *Store) ReorderCategories(ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := make(map[string]int, len(s.data.Categories))
	for i := range s.data.Categories {
		idx[s.data.Categories[i].ID] = i
	}
	changed := false
	for i, id := range ids {
		if pos, ok := idx[id]; ok {
			s.data.Categories[pos].Order = float64(i)
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return s.saveLocked()
}

// ReorderTools 按给定 id 顺序重排工具（同分组内卡片拖拽排序）。
// 只更新出现过的 id 的 Order（按索引），未提及的工具保持原值。
func (s *Store) ReorderTools(ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := make(map[string]int, len(s.data.Tools))
	for i := range s.data.Tools {
		idx[s.data.Tools[i].ID] = i
	}
	changed := false
	for i, id := range ids {
		if pos, ok := idx[id]; ok {
			s.data.Tools[pos].Order = float64(i)
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return s.saveLocked()
}

// ListRecent 返回按最近运行时间倒序排列的前 n 个工具（仅含运行过的）。
func (s *Store) ListRecent(n int) []model.Tool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if n <= 0 {
		n = 12
	}
	cp := make([]model.Tool, 0, len(s.data.Tools))
	for _, t := range s.data.Tools {
		if t.LastRunAt != "" {
			cp = append(cp, t)
		}
	}
	sort.Slice(cp, func(i, j int) bool { return cp[i].LastRunAt > cp[j].LastRunAt })
	if len(cp) > n {
		cp = cp[:n]
	}
	for i := range cp {
		cp[i].HasIcon = s.iconExistsLocked(cp[i].ID)
	}
	return cp
}

// ListCategories 返回**按树序**排列的分类：每个一级分类后紧跟其所有子分类。
// 原版前端侧栏直接按该数组顺序渲染（子项 = 带 parentId 的 rail-sub），
// 因此必须保证“子项紧跟父项”；不能只按 order 全局排序，否则某个排在后面的
// 父目录，其 order 较小的子目录会被排到前面，导致侧栏子项脱离父项错位，
// 也会让“空分类默认收录”误选到子分类。
func (s *Store) ListCategories() []model.Category {
	s.mu.RLock()
	defer s.mu.RUnlock()
	byParent := make(map[string][]model.Category)
	for _, c := range s.data.Categories {
		byParent[c.ParentID] = append(byParent[c.ParentID], c)
	}
	out := make([]model.Category, 0, len(s.data.Categories))
	var walk func(parent string)
	walk = func(parent string) {
		kids := byParent[parent]
		sort.SliceStable(kids, func(i, j int) bool { return kids[i].Order < kids[j].Order })
		for _, c := range kids {
			out = append(out, c)
			walk(c.ID)
		}
	}
	walk("")
	// 兜底：ParentID 指向不存在分类的“孤儿”也原样追加，避免丢分类。
	if len(out) < len(s.data.Categories) {
		seen := make(map[string]bool, len(out))
		for _, c := range out {
			seen[c.ID] = true
		}
		for _, c := range s.data.Categories {
			if !seen[c.ID] {
				out = append(out, c)
			}
		}
	}
	return out
}

// GetTool 按 id 查询单个工具。
func (s *Store) GetTool(id string) (model.Tool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, t := range s.data.Tools {
		if t.ID == id {
			return t, nil
		}
	}
	return model.Tool{}, fmt.Errorf("工具不存在: %s", id)
}

// GetSettings 返回当前设置的副本（切片也复制，避免调用方与内部数据共享底层数组）。
func (s *Store) GetSettings() model.Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := s.data.Settings
	if out.JavaVersions != nil {
		out.JavaVersions = append([]model.RuntimeVersion(nil), out.JavaVersions...)
	}
	if out.PythonVersions != nil {
		out.PythonVersions = append([]model.RuntimeVersion(nil), out.PythonVersions...)
	}
	return out
}

// Snapshot 返回整份数据的深拷贝（备份导出用）。
func (s *Store) Snapshot() model.RegistryData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneData(s.data)
}

// ---------- 写路径 ----------

// AddTool 新增工具并返回落库后的实体（补全 ID / CreatedAt）。
func (s *Store) AddTool(t model.Tool) (model.Tool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t.ID == "" {
		t.ID = NewID()
	}
	if t.CreatedAt == "" {
		t.CreatedAt = time.Now().Format(time.RFC3339)
	}
	s.data.Tools = append(s.data.Tools, t)
	if err := s.saveLocked(); err != nil {
		return model.Tool{}, err
	}
	return t, nil
}

// UpdateTool 更新指定 id 的工具字段（前端弹层总是传全量字段）。
// name/path 为空表示保持原值；categoryID 为空保持原值；envVersion 总是覆盖。
// args/jvmArgs/tags/term/workingDir 仅在“完整编辑”（提供了 name 或 path）时整体覆盖；
// 若 name/path 都为空（前端拖拽改归属就是这种部分更新，传 "" 占位），则保留原启动配置，
// 避免拖拽误清。jvmArgs 仅 jar 工具使用（JVM 选项，拼在 -jar 前）；term=终端运行；
// workingDir=自定义工作目录（空=按类型取默认：jar/bin/sh/py 用工具所在目录）。
func (s *Store) UpdateTool(id, name, path, args, jvmArgs string, tags []string, term bool, categoryID, envVersion, workingDir string) (model.Tool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Tools {
		if s.data.Tools[i].ID != id {
			continue
		}
		t := &s.data.Tools[i]
		if name != "" {
			t.Name = name
		}
		if path != "" {
			t.Path = path
		}
		if name != "" || path != "" {
			t.Args = args // 仅完整编辑时覆盖，允许显式清空
			t.JvmArgs = jvmArgs
			t.Tags = tags
			t.Term = term
			t.WorkingDir = strings.TrimSpace(workingDir)
		}
		if categoryID != "" {
			t.CategoryID = categoryID
		}
		t.EnvVersion = envVersion
		if err := s.saveLocked(); err != nil {
			return model.Tool{}, err
		}
		return *t, nil
	}
	return model.Tool{}, fmt.Errorf("工具不存在: %s", id)
}

// DeleteTool 删除指定工具。
func (s *Store) DeleteTool(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Tools {
		if s.data.Tools[i].ID == id {
			s.data.Tools = append(s.data.Tools[:i], s.data.Tools[i+1:]...)
			return s.saveLocked()
		}
	}
	return fmt.Errorf("工具不存在: %s", id)
}

// ---------- 分类（两级：一级目录 dir / 分组标题 title） ----------
//
// 分类体系规则（对齐原版）：
//   - 空 ParentID = 顶层；非空 = 挂在某父分类下（最多两级）。
//   - Kind:"dir"（空值视作 dir）= 目录，可收纳工具、可含子目录/标题；
//     Kind:"title" = 分组标题，仅作为分区标题，其下不能再建任何内容。
//   - 子目录只能挂在一级目录（顶层 dir）下，即最多两级；
//     标题可挂顶层或一级目录下，但不能挂标题下。
//   - Order 仅在「同一父级」范围内递增，保证同级排序稳定。

// indexOfCatLocked 返回分类在切片中的下标，未找到返回 -1（调用方需持锁）。
func (s *Store) indexOfCatLocked(id string) int {
	for i := range s.data.Categories {
		if s.data.Categories[i].ID == id {
			return i
		}
	}
	return -1
}

// subtreeIDsLocked 返回 root 及其所有后代分类 id 的集合（调用方需持锁）。
func (s *Store) subtreeIDsLocked(root string) map[string]bool {
	set := map[string]bool{root: true}
	for changed := true; changed; {
		changed = false
		for _, c := range s.data.Categories {
			if c.ParentID != "" && set[c.ParentID] && !set[c.ID] {
				set[c.ID] = true
				changed = true
			}
		}
	}
	return set
}

// rootIDLocked 沿 ParentID 向上回溯到顶层分类 id（用于「同一个一级目录范围内移动」）。
func (s *Store) rootIDLocked(id string) string {
	cur := id
	for depth := 0; depth < 16; depth++ {
		i := s.indexOfCatLocked(cur)
		if i < 0 {
			return cur
		}
		p := s.data.Categories[i].ParentID
		if p == "" {
			return cur
		}
		cur = p
	}
	return cur
}

// nextOrderLocked 计算指定父级下新分类的 order（同级最大值 + 1）。
func (s *Store) nextOrderLocked(parentID string) float64 {
	max := float64(-1)
	for _, c := range s.data.Categories {
		if c.ParentID == parentID && c.Order > max {
			max = c.Order
		}
	}
	return max + 1
}

// nameExistsLocked 判断同一父级（同 scope）下是否已存在同名分类。
func (s *Store) nameExistsLocked(name, parentID string) bool {
	for _, c := range s.data.Categories {
		if c.ParentID == parentID && c.Name == name {
			return true
		}
	}
	return false
}

// nameExistsOtherLocked 判断同一父级下是否存在**除 excludeID 之外**的同名分类（重命名查重用）。
func (s *Store) nameExistsOtherLocked(name, parentID, excludeID string) bool {
	for _, c := range s.data.Categories {
		if c.ID != excludeID && c.ParentID == parentID && c.Name == name {
			return true
		}
	}
	return false
}

// appendCategoryLocked 追加分类并落盘。
func (s *Store) appendCategoryLocked(c model.Category) (model.Category, error) {
	s.data.Categories = append(s.data.Categories, c)
	if err := s.saveLocked(); err != nil {
		return model.Category{}, err
	}
	return c, nil
}

// AddCategory 新增顶层目录（kind=dir）。保持单参数签名。
func (s *Store) AddCategory(name string) (model.Category, error) {
	return s.AddSubCategory("", name)
}

// AddSubCategory 在 parentID 下新建子目录（kind=dir）。
//   - parentID 为空：等同新建顶层目录；
//   - 父不存在 → 报错；
//   - 父是标题 → "标题下不能再创建子目录"；
//   - 父本身是二级目录（已有父）→ "目录最多支持两级，无法在二级目录下再新建子目录"；
//   - 同级同名 → "分类已存在"。
func (s *Store) AddSubCategory(parentID, name string) (model.Category, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	name = strings.TrimSpace(name)
	if name == "" {
		return model.Category{}, fmt.Errorf("分类名不能为空")
	}
	if parentID != "" {
		pi := s.indexOfCatLocked(parentID)
		if pi < 0 {
			return model.Category{}, fmt.Errorf("分类不存在: %s", parentID)
		}
		// 分组只挂主分组下（Maye 式布局：分组即页内分节，不再嵌套）。
		if parent := s.data.Categories[pi]; parent.ParentID != "" {
			return model.Category{}, fmt.Errorf("分组下不能再嵌套，请选择主分组")
		}
	}
	if s.nameExistsLocked(name, parentID) {
		return model.Category{}, fmt.Errorf("分类已存在")
	}
	c := model.Category{
		ID:       NewID(),
		Name:     name,
		Order:    s.nextOrderLocked(parentID),
		ParentID: parentID,
		// Kind 留空：目录在原版 JSON 中不带 kind 字段（空值即视作 dir）。
	}
	return s.appendCategoryLocked(c)
}

// RenameCategory 重命名分类。
func (s *Store) RenameCategory(id, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("分类名不能为空")
	}
	i := s.indexOfCatLocked(id)
	if i < 0 {
		return fmt.Errorf("分类不存在: %s", id)
	}
	if s.nameExistsOtherLocked(name, s.data.Categories[i].ParentID, id) {
		return fmt.Errorf("分类已存在")
	}
	s.data.Categories[i].Name = name
	return s.saveLocked()
}

// DeleteCategory 删除分类。
//
// 层级策略（与原版一致）：
//   - 至少保留一个分类，全部删光会被拒绝；
//   - 被删分类若是子级，其工具归入上级分类（上级分类或其他模块）；
//   - 被删父模块的直接子分类提升为顶层，避免产生孤儿分类。
func (s *Store) DeleteCategory(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.data.Categories) <= 1 {
		return fmt.Errorf("至少保留一个分类")
	}
	idx := s.indexOfCatLocked(id)
	if idx < 0 {
		return fmt.Errorf("分类不存在: %s", id)
	}
	parentID := s.data.Categories[idx].ParentID
	fallback := parentID
	if fallback == "" {
		fallback = firstCategoryID(s.data.Categories, id)
	}
	s.data.Categories = append(s.data.Categories[:idx], s.data.Categories[idx+1:]...)
	// 删除父模块时，将其直接子分类提升为顶层，避免孤儿。
	for i := range s.data.Categories {
		if s.data.Categories[i].ParentID == id {
			s.data.Categories[i].ParentID = ""
		}
	}
	for i := range s.data.Tools {
		if s.data.Tools[i].CategoryID == id {
			s.data.Tools[i].CategoryID = fallback
		}
	}
	return s.saveLocked()
}

// UpdateSettings 用 updater 修改设置并原子落盘。
func (s *Store) UpdateSettings(fn func(*model.Settings)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.data.Settings)
	return s.saveLocked()
}

// MarkRun 记录一次工具运行（次数 +1、刷新时间），落盘错误被返回。
func (s *Store) MarkRun(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Tools {
		if s.data.Tools[i].ID == id {
			s.data.Tools[i].RunCount++
			s.data.Tools[i].LastRunAt = time.Now().Format(time.RFC3339Nano)
			return s.saveLocked()
		}
	}
	return nil
}

// RenameTag 将所有工具上的 oldName 标签重命名为 newName（跨工具批量，一处改名全局生效）。
// 重命名后可能与该工具已有标签重合，逐工具去重。
func (s *Store) RenameTag(oldName, newName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" || newName == "" {
		return fmt.Errorf("标签名不能为空")
	}
	if oldName == newName {
		return nil
	}
	found := false
	for i := range s.data.Tools {
		tags := s.data.Tools[i].Tags
		hit := false
		for j, tg := range tags {
			if tg == oldName {
				tags[j] = newName
				hit = true
			}
		}
		if hit {
			found = true
			s.data.Tools[i].Tags = dedupeTags(tags)
		}
	}
	if !found {
		return fmt.Errorf("没有工具使用标签「%s」", oldName)
	}
	return s.saveLocked()
}

// DeleteTag 从所有工具上移除指定标签（跨工具批量）。
func (s *Store) DeleteTag(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("标签名不能为空")
	}
	found := false
	for i := range s.data.Tools {
		tags := s.data.Tools[i].Tags
		out := tags[:0]
		for _, tg := range tags {
			if tg == name {
				found = true
				continue
			}
			out = append(out, tg)
		}
		if found && len(out) != len(tags) {
			s.data.Tools[i].Tags = out
		}
	}
	if !found {
		return fmt.Errorf("没有工具使用标签「%s」", name)
	}
	return s.saveLocked()
}

// dedupeTags 去重并保持原顺序（标签重命名后可能与既有标签重合）。
func dedupeTags(tags []string) []string {
	seen := make(map[string]bool, len(tags))
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		if seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// ReplaceAll 用备份数据整体替换（导入还原时使用）。
func (s *Store) ReplaceAll(d model.RegistryData) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d.Tools == nil {
		d.Tools = []model.Tool{}
	}
	if len(d.Categories) == 0 {
		d.Categories = DefaultCategories()
	}
	s.data = d
	return s.saveLocked()
}

// ---------- 内部辅助 ----------

// iconExistsLocked 判断图标是否存在（调用方需持锁，本方法只读文件系统）。
func (s *Store) iconExistsLocked(id string) bool {
	_, err := os.Stat(IconPathFor(id))
	return err == nil
}

// firstCategoryID 返回除 exclude 外的第一个分类 id，作为兜底分类。
func firstCategoryID(cats []model.Category, exclude string) string {
	for _, c := range cats {
		if c.ID != exclude {
			return c.ID
		}
	}
	return ""
}

// cloneData 深拷贝注册表数据，避免共享底层切片。
func cloneData(d model.RegistryData) model.RegistryData {
	out := model.RegistryData{Settings: d.Settings}
	out.Tools = make([]model.Tool, len(d.Tools))
	copy(out.Tools, d.Tools)
	out.Categories = make([]model.Category, len(d.Categories))
	copy(out.Categories, d.Categories)
	if d.Settings.JavaVersions != nil {
		out.Settings.JavaVersions = make([]model.RuntimeVersion, len(d.Settings.JavaVersions))
		copy(out.Settings.JavaVersions, d.Settings.JavaVersions)
	}
	if d.Settings.PythonVersions != nil {
		out.Settings.PythonVersions = make([]model.RuntimeVersion, len(d.Settings.PythonVersions))
		copy(out.Settings.PythonVersions, d.Settings.PythonVersions)
	}
	return out
}
