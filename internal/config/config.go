// Package config 提供配置类型、加载、迁移、默认值和热重载能力。
//
// 文件组织：
//   - types.go         配置结构体与顺序定义
//   - defaults.go      Default() 默认配置
//   - default_nodes.go 迁移/规范化使用的默认 YAML 节点
//   - provider.go      LLM/VLM provider 预设与独立配置文件解析
//   - load.go          配置加载、默认值补充、版本读取
//   - migrate.go       旧版本配置迁移与 YAML 节点操作
//   - canonical.go     配置顺序规范化
//   - manager.go       配置热重载管理器
package config
