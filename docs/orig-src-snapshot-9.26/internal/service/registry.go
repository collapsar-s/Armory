// Package service 业务服务层。规则：本包及其子包禁止 import Wails（开发计划 §9-4）。
package service

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"armory/internal/model"
)

// Registry 工具收录服务：tools.json 读写 + 分类管理。
type Registry struct {
	mu        sync.RWMutex
	path      string // ~/.armory/tools.json
	Tools     []model.Tool
	Categories []model.Category
	Settings  model.Settings
}

const builtinAll = "all"

var (
	ErrNotFound   = errors.New("工具不存在")
	ErrCatExists  = errors.New("分类已存在")
	ErrCatUsed    = errors.New("分类仍被工具使用")
)

// NewRegistry 打开（或初始化）工具库。dataDir 通常为 ~/.armory。
func NewRegistry(dataDir string) (*Registry, error) {
	r := &Registry{path: filepath.Join(dataDir, "tools.json")}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(r.path)
	switch {
	case os.IsNotExist(err):
		r.Settings = model.Settings{Hotkey: "alt+space", Theme: "auto"}
		r.Categories = defaultCategories()
		if err := r.saveLocked(); err != nil {
			return nil, err
		}
	case err != nil:
		return nil, err
	default:
		if err := json.Unmarshal(data, r); err != nil {
			return nil, errors.New("tools.json 损坏: " + err.Error())
		}
		if r.Settings.Hotkey == "" {
			r.Settings.Hotkey = "alt+space"
		}
		if len(r.Categories) == 0 {
			r.Categories = defaultCategories()
		}
	}
	return r, nil
}

func defaultCategories() []model.Category {
	names := []string{"信息收集", "漏洞利用", "代理隧道", "开发日常"}
	out := make([]model.Category, 0, len(names))
	for i, n := range names {
		out = append(out, model.Category{ID: "cat-" + n, Name: n, Order: i})
	}
	return out
}

func (r *Registry) saveLocked() error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, r.path) // 原子写，防损坏
}

func NewID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ExpandPath 展开 ~ 前缀。
func ExpandPath(p string) string {
	if strings.HasPrefix(p, "~") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	return p
}

/* ---------------- 工具 CRUD ---------------- */

// ListTools 返回分类下全部工具（builtinAll 返回全部）。
func (r *Registry) ListTools(categoryID string) []model.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]model.Tool, 0, len(r.Tools))
	for _, t := range r.Tools {
		if categoryID == "" || categoryID == builtinAll || t.CategoryID == categoryID {
			out = append(out, t)
		}
	}
	return out
}

// AddTool 收录工具（去重：同路径同名加序号）。
func (r *Registry) AddTool(t model.Tool) (model.Tool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t.Name == "" || t.Path == "" || t.Type == "" {
		return model.Tool{}, errors.New("name/path/type 不能为空")
	}
	if t.CategoryID == "" || t.CategoryID == builtinAll {
		t.CategoryID = firstCategoryID(r.Categories)
	}
	t.ID = NewID()
	t.CreatedAt = time.Now().Format(time.RFC3339)
	// 同分类重名处理
	n, base := 1, t.Name
	for containsName(r.Tools, t.CategoryID, t.Name) {
		n++
		t.Name = base + " " + itoa(n)
	}
	r.Tools = append(r.Tools, t)
	return t, r.saveLocked()
}

// GetTool 按 ID 取单个工具。
func (r *Registry) GetTool(id string) (model.Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, t := range r.Tools {
		if t.ID == id {
			return t, nil
		}
	}
	return model.Tool{}, ErrNotFound
}

// UpdateTool 按.ID 覆盖可编辑字段。
func (r *Registry) UpdateTool(id string, fn func(t *model.Tool)) (model.Tool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.Tools {
		if r.Tools[i].ID == id {
			fn(&r.Tools[i])
			out := r.Tools[i]
			return out, r.saveLocked()
		}
	}
	return model.Tool{}, ErrNotFound
}

// DeleteTool 删除工具。
func (r *Registry) DeleteTool(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.Tools {
		if r.Tools[i].ID == id {
			r.Tools = append(r.Tools[:i], r.Tools[i+1:]...)
			return r.saveLocked()
		}
	}
	return ErrNotFound
}

// MarkRun 记录一次启动（更新 lastRunAt / runCount）。
func (r *Registry) MarkRun(id string) error {
	_, err := r.UpdateTool(id, func(t *model.Tool) {
		t.LastRunAt = time.Now().Format(time.RFC3339)
		t.RunCount++
	})
	return err
}

// ListRecent 最近使用（M4-2）：按 lastRunAt 倒序取前 n 个有启动记录的工具。
func (r *Registry) ListRecent(n int) []model.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]model.Tool, 0, n)
	for _, t := range r.Tools {
		if t.LastRunAt != "" {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastRunAt > out[j].LastRunAt })
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// ReplaceAll 整库替换（M4-4 导入还原）：工具/分类/设置一次覆盖并落盘。
func (r *Registry) ReplaceAll(tools []model.Tool, cats []model.Category, settings model.Settings) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Tools = tools
	r.Categories = cats
	r.Settings = settings
	if r.Settings.Hotkey == "" {
		r.Settings.Hotkey = "alt+space"
	}
	return r.saveLocked()
}

/* ---------------- 分类管理 ---------------- */

func (r *Registry) ListCategories() []model.Category {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := append([]model.Category(nil), r.Categories...)
	sort.Slice(out, func(i, j int) bool { return out[i].Order < out[j].Order })
	return out
}

func (r *Registry) AddCategory(name string) (model.Category, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.Categories {
		if c.Name == name {
			return model.Category{}, ErrCatExists
		}
	}
	c := model.Category{ID: "cat-" + NewID(), Name: name, Order: len(r.Categories)}
	r.Categories = append(r.Categories, c)
	return c, r.saveLocked()
}

func (r *Registry) RenameCategory(id, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.Categories {
		if r.Categories[i].ID == id {
			r.Categories[i].Name = name
			return r.saveLocked()
		}
	}
	return ErrNotFound
}

// DeleteCategory 删除分类；分类下工具移入第一个剩余分类。
func (r *Registry) DeleteCategory(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.Categories) <= 1 {
		return errors.New("至少保留一个分类")
	}
	fallback := ""
	for _, c := range r.Categories {
		if c.ID != id {
			fallback = c.ID
			break
		}
	}
	out := r.Categories[:0]
	for _, c := range r.Categories {
		if c.ID != id {
			out = append(out, c)
		}
	}
	r.Categories = out
	for i := range r.Tools {
		if r.Tools[i].CategoryID == id {
			r.Tools[i].CategoryID = fallback
		}
	}
	return r.saveLocked()
}

/* ---------------- 设置 ---------------- */

func (r *Registry) GetSettings() model.Settings {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Settings
}

func (r *Registry) UpdateSettings(fn func(s *model.Settings)) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	fn(&r.Settings)
	return r.saveLocked()
}

/* ---------------- helpers ---------------- */

func containsName(tools []model.Tool, cat, name string) bool {
	for _, t := range tools {
		if t.CategoryID == cat && t.Name == name {
			return true
		}
	}
	return false
}

func firstCategoryID(cats []model.Category) string {
	if len(cats) == 0 {
		return ""
	}
	min := cats[0]
	for _, c := range cats {
		if c.Order < min.Order {
			min = c
		}
	}
	return min.ID
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
