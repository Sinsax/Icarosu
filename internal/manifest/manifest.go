package manifest

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"icarosu/internal/sdk"
)

// Parse 解析并校验插件 manifest。
func Parse(data []byte) (sdk.Manifest, error) {
	var m sdk.Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return sdk.Manifest{}, fmt.Errorf("parse manifest: %w", err)
	}
	if err := m.Validate(); err != nil {
		return sdk.Manifest{}, fmt.Errorf("validate manifest: %w", err)
	}
	return m, nil
}

// MustParse 解析失败时 panic，适合 go:embed 的静态文件。
func MustParse(data []byte) sdk.Manifest {
	m, err := Parse(data)
	if err != nil {
		panic(err)
	}
	return m
}
