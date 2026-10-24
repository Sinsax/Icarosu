// Package store 提供 SQLite 长期存储与领域 Repository 接口。
//
// 文件组织：
//   - db.go        数据库打开/关闭/迁移
//   - schema.go    schema 初始化语句
//   - models.go    存储层数据结构
//   - *_repo.go    按领域拆分的 Repository 实现
//   - repository.go 供上层使用的 Repository 接口
package store