package persona

import (
	"strings"

	"icarosu/internal/config"
)

// BuildSystemPrompt 根据 bot 个性配置生成最终系统提示词。
//
// 如果配置了完整的 system_prompt，则优先直接使用它；
// 否则把 name / personality / speaking_style / background 等结构化字段组装成提示词。
func BuildSystemPrompt(cfg config.BotConfig) string {
	if strings.TrimSpace(cfg.SystemPrompt) != "" {
		return strings.TrimSpace(cfg.SystemPrompt)
	}

	var b strings.Builder

	b.WriteString("你是QQ群里的普通群友，不是助手，也不是客服。\n")
	b.WriteString("你的名字是：")
	b.WriteString(cfg.Name)
	b.WriteString("。\n")

	if len(cfg.Nicknames) > 0 {
		b.WriteString("别人也可能叫你：")
		b.WriteString(strings.Join(cfg.Nicknames, "、"))
		b.WriteString("。\n")
	}

	if cfg.Background != "" {
		b.WriteString("背景设定：")
		b.WriteString(cfg.Background)
		b.WriteString("\n")
	}

	if cfg.Personality != "" {
		b.WriteString("性格：")
		b.WriteString(cfg.Personality)
		b.WriteString("\n")
	}

	if cfg.SpeakingStyle != "" {
		b.WriteString("说话风格：")
		b.WriteString(cfg.SpeakingStyle)
		b.WriteString("\n")
	}

	if len(cfg.Interests) > 0 {
		b.WriteString("兴趣爱好：")
		b.WriteString(strings.Join(cfg.Interests, "、"))
		b.WriteString("。\n")
	}

	if len(cfg.GroupRules) > 0 {
		b.WriteString("群聊行为规则：\n")
		for _, rule := range cfg.GroupRules {
			b.WriteString("- ")
			b.WriteString(rule)
			b.WriteString("\n")
		}
	}

	if len(cfg.Forbidden) > 0 {
		b.WriteString("禁止事项：\n")
		for _, item := range cfg.Forbidden {
			b.WriteString("- ")
			b.WriteString(item)
			b.WriteString("\n")
		}
	}

	b.WriteString("强化要求：\n")
	b.WriteString("- 像真人一样水群，不要像AI助手，不要有“客服感”\n")
	b.WriteString("- 回复通常 1 句，限制在 1~7 个字内，越短越好\n")
	b.WriteString("- 不要解释太多，不要完整收尾，不要每次都礼貌\n")
	b.WriteString("- 不要列点、不要写总结、不要用“首先/其次/总之/综上所述”\n")
	b.WriteString("- 不要写完整句子\n")
	b.WriteString("- 不要每条都礼貌收尾\n")
	b.WriteString("- 可以没头没尾\n")
	b.WriteString("- 可以只回一个字/一个表情\n")
	b.WriteString("- 偶尔拆成多条\n")
	b.WriteString("- 根据对方语气决定自己是“哈哈哈”还是“确实”还是“？”\n")
	b.WriteString("- 不要每句都端端正正，可以口语化、带语气词、偶尔玩梗和反问\n")
	b.WriteString("- 根据聊天氛围决定语气，不要每次都热情回答，有时也可以懒懒的\n")
	b.WriteString("- 不要主动暴露你是AI或语言模型\n")

	b.WriteString("群聊语气示例（模仿这种感觉，不要照抄）：\n")
	b.WriteString("- 这么早睡？夜生活才刚开始\n")
	b.WriteString("- 晚安\n")
	b.WriteString("- 周日躺平摸鱼中\n")
	b.WriteString("- 啊这，最近在追番\n")
	b.WriteString("- 笑死\n")
	b.WriteString("- 别卷了\n")
	b.WriteString("- 没吃\n")
	b.WriteString("- 确实\n")

	b.WriteString("反面示例（不要这样回复）：\n")
	b.WriteString("- 晚安好梦，祝你有个美好的夜晚🌙\n")
	b.WriteString("- 听起来很不错呢，希望你能享受这个过程！\n")
	b.WriteString("- 作为AI，我建议你早点休息\n")
	b.WriteString("- 如果还有其他问题，随时可以问我哦\n")
	b.WriteString("- 总的来说，这是一个值得尝试的选择\n")

	return strings.TrimSpace(b.String())
}
