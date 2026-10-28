// Command icarosu 是 Go SDK 插件化运行时的新入口。
// 它使用 platform-neutral kernel + standard bundle + OneBot adapter；
// 旧 cmd/bot 仍保留，便于回退。
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"

	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"

	"icarosu/bundles/standard"
	"icarosu/internal/config"
	"icarosu/internal/kernel"
	"icarosu/internal/llm"
	"icarosu/internal/logger"
	"icarosu/internal/meme"
	"icarosu/internal/plugins/adapter/onebot"
	adminplugin "icarosu/internal/plugins/interface/admin"
	webuiplugin "icarosu/internal/plugins/interface/webui"
	"icarosu/internal/qq"
	"icarosu/internal/sdk"
	"icarosu/internal/vlm"
)

func main() {
	var configPath string
	var pluginsDir string
	var kernelConfigPath string
	var migrateConfig bool
	flag.StringVar(&configPath, "config", "configs/config.yaml", "path to config file")
	flag.StringVar(&pluginsDir, "plugins-dir", "", "plugin config directory (default: <config dir>/plugins)")
	flag.StringVar(&kernelConfigPath, "kernel-config", "", "path to kernel.yaml (overrides config/plugins-dir)")
	flag.BoolVar(&migrateConfig, "migrate-config", false, "write split plugin config files and exit")
	flag.Parse()

	var kf KernelFile
	var enabled map[string]bool
	if kernelConfigPath != "" {
		var err error
		kf, err = loadKernelFile(kernelConfigPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load kernel config: %v\n", err)
			os.Exit(1)
		}
		base := filepath.Dir(kernelConfigPath)
		if kf.Kernel.Config != "" {
			configPath = resolvePath(base, kf.Kernel.Config)
		}
		if pluginsDir == "" && kf.Kernel.PluginsDir != "" {
			pluginsDir = resolvePath(base, kf.Kernel.PluginsDir)
		}
		enabled = kf.enabledSet()
	}
	if pluginsDir == "" {
		pluginsDir = filepath.Join(filepath.Dir(configPath), "plugins")
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}
	if migrateConfig {
		if err := writePluginConfigs(cfg, pluginsDir); err != nil {
			fmt.Fprintf(os.Stderr, "migrate plugin configs: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "wrote plugin configs to %s\n", pluginsDir)
		return
	}

	if cfg.DryRun {
		fmt.Fprintln(os.Stderr, "dry_run is not supported by the sdk runtime; refusing to start to avoid sending real messages")
		os.Exit(1)
	}
	log, _ := logger.NewWithBuffer(cfg.Log.Level, 1000)

	llmClient := llm.NewClient(llm.Config{
		BaseURL:        cfg.LLM.BaseURL,
		APIKey:         cfg.LLM.APIKey,
		Model:          cfg.LLM.Model,
		Temperature:    cfg.LLM.Temperature,
		MaxTokens:      cfg.LLM.MaxTokens,
		Timeout:        time.Duration(cfg.LLM.TimeoutSeconds) * time.Second,
		CachePrefix:    cfg.LLM.CachePrefix,
		CacheMaxTokens: cfg.LLM.CacheMaxTokens,
		LogCache:       cfg.LLM.LogCache,
	}, log)

	var vision sdk.Vision
	var memes sdk.MemeStore
	if cfg.VLM.BaseURL != "" && cfg.VLM.Model != "" {
		vlmClient := vlm.NewClient(vlm.Config{
			BaseURL: cfg.VLM.BaseURL,
			APIKey:  cfg.VLM.APIKey,
			Model:   cfg.VLM.Model,
			Prompt:  cfg.VLM.Prompt,
			Timeout: time.Duration(cfg.VLM.TimeoutSeconds) * time.Second,
		}, log)
		vision = vlm.NewSDKVision(vlmClient)
		if store, err := meme.Open(cfg.Meme.Directory); err != nil {
			log.Warn("open meme store failed, steal disabled", "error", err)
		} else {
			memes = store
		}
	}

	onebotClient := qq.NewClient(cfg.QQ.OneBot.WSURL, cfg.QQ.OneBot.AccessToken, log)
	adapterPlugin := onebot.NewPluginWithAdmins(onebotClient, cfg.Bot.AdminIDs, log)

	if enabled != nil {
		for _, id := range []string{"adapter.onebot", "governance.filter", "governance.safety", "persona.default", "renderer.default", "drive.attention"} {
			if !enabled[id] {
				fmt.Fprintf(os.Stderr, "kernel config must enable critical plugin %q\n", id)
				os.Exit(1)
			}
		}
	}

	workers := cfg.Queue.Workers
	if kf.Kernel.SchedulerWorkers > 0 {
		workers = kf.Kernel.SchedulerWorkers
	}
	buffer := cfg.Queue.Buffer
	if kf.Kernel.SchedulerBuffer > 0 {
		buffer = kf.Kernel.SchedulerBuffer
	}

	k := kernel.New(kernel.Options{
		Logger:           log,
		LLM:              llm.NewSDKClient(llmClient),
		Vision:           vision,
		Memes:            memes,
		Timeout:          20 * time.Second,
		SchedulerWorkers: workers,
		SchedulerBuffer:  buffer,
		Config: func(section string, dst any) error {
			return loadSection(cfg, pluginsDir, section, dst)
		},
	})
	register := func(spec sdk.PluginSpec, id string) error {
		if enabled != nil && !enabled[id] {
			return nil
		}
		return k.Register(spec)
	}
	for _, spec := range standard.Plugins() {
		plugin := spec.New()
		id := plugin.Manifest().ID
		if enabled != nil && !enabled[id] {
			continue
		}
		spec := sdk.PluginSpec{New: func() sdk.Plugin { return plugin }, Critical: spec.Critical}
		if err := k.Register(spec); err != nil {
			fmt.Fprintf(os.Stderr, "register standard plugin %s: %v\n", id, err)
			os.Exit(1)
		}
	}
	if err := register(sdk.PluginSpec{New: func() sdk.Plugin { return adapterPlugin }, Critical: true}, "adapter.onebot"); err != nil {
		fmt.Fprintf(os.Stderr, "register onebot adapter: %v\n", err)
		os.Exit(1)
	}
	if err := register(sdk.PluginSpec{New: func() sdk.Plugin { return webuiplugin.New(k, k, pluginsDir, log) }, Critical: false}, "interface.webui"); err != nil {
		fmt.Fprintf(os.Stderr, "register webui plugin: %v\n", err)
		os.Exit(1)
	}
	if err := register(sdk.PluginSpec{New: func() sdk.Plugin { return adminplugin.New(k, log) }, Critical: false}, "interface.admin"); err != nil {
		fmt.Fprintf(os.Stderr, "register admin plugin: %v\n", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := k.Init(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "init kernel: %v\n", err)
		os.Exit(1)
	}
	if err := k.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "start kernel: %v\n", err)
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
		_ = k.Stop(stopCtx)
		stopCancel()
		os.Exit(1)
	}
	log.Info("sdk runtime started", "ws_url", cfg.QQ.OneBot.WSURL, "model", cfg.LLM.Model)

	<-ctx.Done()
	log.Info("shutting down sdk runtime")
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer stopCancel()
	if err := k.Stop(stopCtx); err != nil {
		log.Warn("stop kernel", "error", err)
	}
}

func writePluginConfigs(cfg *config.Config, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	files := map[string]any{
		"persona.default": map[string]any{
			"version": 1,
			"enabled": true,
			"config": map[string]any{
				"name":           cfg.Bot.Name,
				"nicknames":      cfg.Bot.Nicknames,
				"personality":    cfg.Bot.Personality,
				"speaking_style": cfg.Bot.SpeakingStyle,
				"background":     cfg.Bot.Background,
			},
		},
		"drive.attention": map[string]any{
			"version": 1,
			"enabled": true,
			"config": map[string]any{
				"keywords":           cfg.Bot.Keywords,
				"random_probability": cfg.Bot.RandomProbability,
			},
		},
		"governance.filter": map[string]any{
			"version": 1,
			"enabled": true,
			"config": map[string]any{
				"enabled":   cfg.Filter.Enabled,
				"mode":      cfg.Filter.Mode,
				"group_ids": cfg.Filter.GroupIDs,
				"user_ids":  cfg.Filter.UserIDs,
			},
		},
		"governance.safety": map[string]any{
			"version": 1,
			"enabled": true,
			"config":  map[string]any{"forbidden": cfg.Bot.Forbidden},
		},
		"interface.webui": map[string]any{
			"version": 1,
			"enabled": cfg.WebUI.Enabled,
			"config": map[string]any{
				"enabled": cfg.WebUI.Enabled,
				"addr":    cfg.WebUI.Addr,
				"token":   cfg.WebUI.Token,
			},
		},
	}
	for name, value := range files {
		path := filepath.Join(dir, name+".yaml")
		if _, err := os.Stat(path); err == nil {
			fmt.Fprintf(os.Stderr, "skip existing plugin config %s\n", path)
			continue // 不覆盖用户已迁移/手工编辑的插件配置
		}
		data, err := yaml.Marshal(value)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func loadPluginConfigFile(dir, section string, dst any) error {
	if dir == "" {
		return os.ErrNotExist
	}
	data, err := os.ReadFile(filepath.Join(dir, section+".yaml"))
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return err
	}
	if len(doc.Content) > 0 {
		root := doc.Content[0]
		if root.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(root.Content); i += 2 {
				if root.Content[i].Value == "config" {
					return decodeYAML(root.Content[i+1], dst)
				}
			}
		}
	}
	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}
	return decodeJSON(raw, dst)
}

func decodeYAML(node *yaml.Node, dst any) error {
	var raw any
	if err := node.Decode(&raw); err != nil {
		return err
	}
	return decodeJSON(raw, dst)
}

func decodeJSON(raw any, dst any) error {
	data, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}

func loadSection(cfg *config.Config, pluginsDir, section string, dst any) error {
	if err := loadPluginConfigFile(pluginsDir, section, dst); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	var value any
	switch section {
	case "persona.default":
		value = map[string]any{
			"name":           cfg.Bot.Name,
			"nicknames":      cfg.Bot.Nicknames,
			"personality":    cfg.Bot.Personality,
			"speaking_style": cfg.Bot.SpeakingStyle,
			"background":     cfg.Bot.Background,
		}
	case "drive.attention":
		value = map[string]any{
			"keywords":           cfg.Bot.Keywords,
			"random_probability": cfg.Bot.RandomProbability,
		}
	case "governance.filter":
		value = map[string]any{
			"enabled":   cfg.Filter.Enabled,
			"mode":      cfg.Filter.Mode,
			"group_ids": cfg.Filter.GroupIDs,
			"user_ids":  cfg.Filter.UserIDs,
		}
	case "governance.safety":
		value = map[string]any{"forbidden": cfg.Bot.Forbidden}
	case "interface.webui":
		value = map[string]any{
			"enabled": cfg.WebUI.Enabled,
			"addr":    cfg.WebUI.Addr,
			"token":   cfg.WebUI.Token,
		}
	default:
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}
