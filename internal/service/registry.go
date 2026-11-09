// Package service 实现 Armory 的业务逻辑层：工具注册表、启动器、图标提取、
// 运行时探测、说明文档解析、备份导入导出，以及暴露给前端（Wails bindings）的 API。
//
// 本包将持久化细节委托给 internal/store，自身专注于业务编排与前端契约。
package service

import (
	"armory/internal/model"
	"armory/internal/store"
)

// Registry 是业务层对持久化层的封装，同时作为 API 依赖注入的入口。
// 它对外暴露工具/分类/设置的查询与变更能力，内部把并发与落盘交给 store。
type Registry struct {
	st *store.Store
}

// NewRegistry 打开（或初始化）~/.armory/tools.json 并返回 Registry。
func NewRegistry() (*Registry, error) {
	st, err := store.New()
	if err != nil {
		return nil, err
	}
	return &Registry{st: st}, nil
}

// ListTools 返回指定分类下的工具；categoryID 为空或 "all" 时返回全部。
func (r *Registry) ListTools(categoryID string) []model.Tool { return r.st.ListTools(categoryID) }

// ListRecent 返回最近使用的 n 个工具。
func (r *Registry) ListRecent(n int) []model.Tool { return r.st.ListRecent(n) }

// ListCategories 返回按 order 排序的分类。
func (r *Registry) ListCategories() []model.Category { return r.st.ListCategories() }

// GetTool 按 id 查询单个工具。
func (r *Registry) GetTool(id string) (model.Tool, error) { return r.st.GetTool(id) }

// AddTool 新增工具。
func (r *Registry) AddTool(t model.Tool) (model.Tool, error) { return r.st.AddTool(t) }

// UpdateTool 更新工具字段。
func (r *Registry) UpdateTool(id, name, path, args, jvmArgs string, tags []string, term bool, categoryID, envVersion, workingDir string) (model.Tool, error) {
	return r.st.UpdateTool(id, name, path, args, jvmArgs, tags, term, categoryID, envVersion, workingDir)
}

// DeleteTool 删除工具。
func (r *Registry) DeleteTool(id string) error { return r.st.DeleteTool(id) }

// AddCategory 新增顶层目录（kind=dir），保持单参数签名。
func (r *Registry) AddCategory(name string) (model.Category, error) {
	return r.st.AddCategory(name)
}

// AddSubCategory 在 parentID 下新建分组。
func (r *Registry) AddSubCategory(parentID, name string) (model.Category, error) {
	return r.st.AddSubCategory(parentID, name)
}

// ReorderCategories 按给定 id 顺序重排分类。
func (r *Registry) ReorderCategories(ids []string) error { return r.st.ReorderCategories(ids) }

// ReorderTools 按给定 id 顺序重排工具。
func (r *Registry) ReorderTools(ids []string) error { return r.st.ReorderTools(ids) }

// RenameTag 跨工具重命名标签。
func (r *Registry) RenameTag(oldName, newName string) error { return r.st.RenameTag(oldName, newName) }

// DeleteTag 从所有工具上移除标签。
func (r *Registry) DeleteTag(name string) error { return r.st.DeleteTag(name) }

// RenameCategory 重命名分类。
func (r *Registry) RenameCategory(id, name string) error { return r.st.RenameCategory(id, name) }

// DeleteCategory 删除分类。
func (r *Registry) DeleteCategory(id string) error { return r.st.DeleteCategory(id) }

// GetSettings 返回当前设置。
func (r *Registry) GetSettings() model.Settings { return r.st.GetSettings() }

// UpdateSettings 修改设置。
func (r *Registry) UpdateSettings(fn func(*model.Settings)) error { return r.st.UpdateSettings(fn) }

// MarkRun 记录一次运行。
func (r *Registry) MarkRun(id string) error { return r.st.MarkRun(id) }

// ReplaceAll 整体替换数据（导入还原）。
func (r *Registry) ReplaceAll(d model.RegistryData) error { return r.st.ReplaceAll(d) }

// Snapshot 返回整份数据快照（备份导出）。
func (r *Registry) Snapshot() model.RegistryData { return r.st.Snapshot() }
