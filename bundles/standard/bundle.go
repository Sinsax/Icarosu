package standard

import (
	attention "icarosu/internal/plugins/drive/attention"
	filter "icarosu/internal/plugins/governance/filter"
	safety "icarosu/internal/plugins/governance/safety"
	recent "icarosu/internal/plugins/memory/recent"
	persona "icarosu/internal/plugins/persona/default"
	jargonlearn "icarosu/internal/plugins/reflection/jargonlearn"
	membersync "icarosu/internal/plugins/reflection/membersync"
	replylog "icarosu/internal/plugins/reflection/replylog"
	steal "icarosu/internal/plugins/reflection/steal"
	stylelearn "icarosu/internal/plugins/reflection/stylelearn"
	summarizer "icarosu/internal/plugins/reflection/summarizer"
	renderer "icarosu/internal/plugins/renderer/default"
	system "icarosu/internal/plugins/tool/system"
	timeplugin "icarosu/internal/plugins/tool/time"
	"icarosu/internal/sdk"
)

// Plugins 返回第一方标准包（不含平台 adapter，adapter 由 app 按配置注入）。
func Plugins() []sdk.PluginSpec {
	return []sdk.PluginSpec{
		{New: filter.New, Critical: true},
		{New: safety.New, Critical: true},
		{New: persona.New, Critical: true},
		{New: renderer.New, Critical: true},
		{New: attention.New, Critical: true},
		{New: recent.New, Critical: false},
		{New: replylog.New, Critical: false},
		{New: summarizer.New, Critical: false},
		{New: stylelearn.New, Critical: false},
		{New: jargonlearn.New, Critical: false},
		{New: membersync.New, Critical: false},
		{New: steal.New, Critical: false},
		{New: system.New, Critical: false},
		{New: timeplugin.New, Critical: false},
	}
}
