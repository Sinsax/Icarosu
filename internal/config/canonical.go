package config

import "gopkg.in/yaml.v3"

func canonicalize(root *yaml.Node) {
	reorderMapping(root, topLevelOrder)

	if botNode := findMappingValue(root, "bot"); botNode != nil {
		reorderMapping(botNode, botOrder)
	}
	if filterNode := findMappingValue(root, "filter"); filterNode != nil {
		reorderMapping(filterNode, filterOrder)
	}
	if memoryNode := findMappingValue(root, "memory"); memoryNode != nil {
		reorderMapping(memoryNode, memoryOrder)
	}
	if rateLimitNode := findMappingValue(root, "rate_limit"); rateLimitNode != nil {
		reorderMapping(rateLimitNode, rateLimitOrder)
	}
	if queueNode := findMappingValue(root, "queue"); queueNode != nil {
		reorderMapping(queueNode, queueOrder)
	}
	if storeNode := findMappingValue(root, "store"); storeNode != nil {
		reorderMapping(storeNode, storeOrder)
	}
	if vlmNode := findMappingValue(root, "vlm"); vlmNode != nil {
		reorderMapping(vlmNode, vlmOrder)
	}
	if memeNode := findMappingValue(root, "meme"); memeNode != nil {
		reorderMapping(memeNode, memeOrder)
	}
	if attentionNode := findMappingValue(root, "attention"); attentionNode != nil {
		reorderMapping(attentionNode, attentionOrder)
	}
	if webuiNode := findMappingValue(root, "webui"); webuiNode != nil {
		reorderMapping(webuiNode, webuiOrder)
	}
	if qqNode := findMappingValue(root, "qq"); qqNode != nil {
		reorderMapping(qqNode, qqOrder)
		if onebotNode := findMappingValue(qqNode, "onebot"); onebotNode != nil {
			reorderMapping(onebotNode, onebotOrder)
		}
	}
	if llmNode := findMappingValue(root, "llm"); llmNode != nil {
		reorderMapping(llmNode, llmOrder)
	}
	if logNode := findMappingValue(root, "log"); logNode != nil {
		reorderMapping(logNode, logOrder)
	}
}

// isCanonical 检查当前配置是否已经符合预设顺序。
func isCanonical(root *yaml.Node) bool {
	return mappingHasOrder(root, topLevelOrder) &&
		mappingHasOrder(findMappingValue(root, "bot"), botOrder) &&
		mappingHasOrder(findMappingValue(root, "filter"), filterOrder) &&
		mappingHasOrder(findMappingValue(root, "memory"), memoryOrder) &&
		mappingHasOrder(findMappingValue(root, "rate_limit"), rateLimitOrder) &&
		mappingHasOrder(findMappingValue(root, "queue"), queueOrder) &&
		mappingHasOrder(findMappingValue(root, "store"), storeOrder) &&
		mappingHasOrder(findMappingValue(root, "vlm"), vlmOrder) &&
		mappingHasOrder(findMappingValue(root, "meme"), memeOrder) &&
		mappingHasOrder(findMappingValue(root, "attention"), attentionOrder) &&
		mappingHasOrder(findMappingValue(root, "webui"), webuiOrder) &&
		mappingHasOrder(findMappingValue(root, "qq"), qqOrder) &&
		mappingHasOrder(findMappingValue(findMappingValue(root, "qq"), "onebot"), onebotOrder) &&
		mappingHasOrder(findMappingValue(root, "llm"), llmOrder) &&
		mappingHasOrder(findMappingValue(root, "log"), logOrder)
}

func reorderMapping(mapping *yaml.Node, order []string) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return
	}

	index := make(map[string]int, len(mapping.Content)/2)
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		index[mapping.Content[i].Value] = i
	}

	ordered := make([]*yaml.Node, 0, len(mapping.Content))
	seen := make(map[string]bool, len(mapping.Content)/2)
	for _, key := range order {
		i, ok := index[key]
		if !ok {
			continue
		}
		ordered = append(ordered, mapping.Content[i], mapping.Content[i+1])
		seen[key] = true
	}

	// 保留未知/自定义字段，放在末尾。
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if !seen[mapping.Content[i].Value] {
			ordered = append(ordered, mapping.Content[i], mapping.Content[i+1])
		}
	}

	mapping.Content = ordered
}

func mappingHasOrder(mapping *yaml.Node, order []string) bool {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return true
	}

	pos := make(map[string]int, len(order))
	for i, key := range order {
		pos[key] = i
	}

	last := -1
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		key := mapping.Content[i].Value
		p, ok := pos[key]
		if !ok {
			continue
		}
		if p < last {
			return false
		}
		last = p
	}
	return true
}
