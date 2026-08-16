package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"icarosu/internal/attention"
	"icarosu/internal/config"
	"icarosu/internal/handler"
	"icarosu/internal/llm"
	"icarosu/internal/logger"
	"icarosu/internal/meme"
	"icarosu/internal/memory"
	"icarosu/internal/persona"
	"icarosu/internal/plugin"
	"icarosu/internal/plugin/admin"
	_ "icarosu/internal/plugin/builtin"
	"icarosu/internal/plugin/chat"
	"icarosu/internal/plugin/filter"
	"icarosu/internal/plugin/membersync"
	"icarosu/internal/plugin/recorder"
	"icarosu/internal/plugin/steal"
	"icarosu/internal/plugin/stylelearn"
	"icarosu/internal/plugin/summarizer"
	"icarosu/internal/plugin/trigger"
	"icarosu/internal/qq"
	"icarosu/internal/ratelimit"
	"icarosu/internal/store"
	"icarosu/internal/style"
	"icarosu/internal/tools"
	"icarosu/internal/vision"
	"icarosu/internal/vlm"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "configs/config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.Log.Level)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// LLM 客户端：OpenAI 兼容接口，支持 DeepSeek / 通义 / 智谱 / Ollama 等。
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
	log.Info("llm configured",
		"config_file", cfg.LLMConfigFile,
		"provider", cfg.LLM.Provider,
		"base_url", cfg.LLM.BaseURL,
		"model", cfg.LLM.Model,
		"api_key_set", cfg.LLM.APIKey != "",
	)

	// 长期存储与记忆服务
	var st *store.Store
	if cfg.Memory.LongTermEnabled {
		st, err = store.Open(cfg.Store)
		if err != nil {
			log.Error("failed to open store", "error", err)
			os.Exit(1)
		}
		defer func() {
			_ = st.Close()
		}()
	}
	mem := memory.New(cfg.Memory.ShortTermSize, st)
	mem.SetContextMaxAge(cfg.Memory.ContextMaxAgeMinutes)
	mem.PreloadFromStore(cfg.Memory.ShortTermSize)

	attentionTracker := attention.New(cfg.Attention)

	// VLM 通用图片识别链路：只要配置了 vlm 且能连通，就自动开启“图片理解 + 表情库”
	var memeStore *meme.Store
	var visionSvc *vision.Service
	if cfg.VLM.BaseURL != "" && cfg.VLM.Model != "" {
		vlmClient := vlm.NewClient(vlm.Config{
			BaseURL: cfg.VLM.BaseURL,
			APIKey:  cfg.VLM.APIKey,
			Model:   cfg.VLM.Model,
			Prompt:  cfg.VLM.Prompt,
			Timeout: time.Duration(cfg.VLM.TimeoutSeconds) * time.Second,
		}, log)

		if err := vlmClient.Ping(ctx); err != nil {
			log.Warn("vlm not available, image chain disabled", "error", err)
		} else {
			memeStore, err = meme.Open(cfg.Meme.Directory)
			if err != nil {
				log.Error("failed to open meme store", "error", err)
				os.Exit(1)
			}
			visionSvc = vision.New(vlmClient, memeStore, cfg.Meme)
			log.Info("vlm image understanding enabled", "model", cfg.VLM.Model)
		}
	}

	personaPrompt := persona.BuildSystemPrompt(cfg.Bot)

	client := qq.NewClient(cfg.QQ.OneBot.WSURL, cfg.QQ.OneBot.AccessToken, log)

	// 插件管理器：后续所有扩展（AI 聊天、管理、娱乐等）都以插件形式注册到这里。
	// 简单插件可通过 init() 自注册到全局注册表；需要依赖的插件在 main 中显式构造注册。
	manager := plugin.NewManager()
	for _, p := range plugin.GlobalPlugins() {
		manager.Register(p)
	}
	manager.Register(filter.New(cfg.Filter, filter.WithLogger(log)))
	recorderOpts := []recorder.Option{recorder.WithLogger(log)}
	if st != nil {
		recorderOpts = append(recorderOpts, recorder.WithStore(st))
	}
	if visionSvc != nil {
		recorderOpts = append(recorderOpts, recorder.WithVision(visionSvc))
	}
	manager.Register(recorder.New(mem, recorderOpts...))
	if st != nil {
		manager.Register(membersync.New(client, st, membersync.WithLogger(log)))
	}
	if memeStore != nil && visionSvc != nil {
		manager.Register(steal.New(cfg.Meme, visionSvc, personaPrompt, steal.WithLogger(log)))
	}
	if st != nil && llmClient != nil {
		manager.Register(summarizer.New(llmClient, mem, st, cfg.Memory, summarizer.WithLogger(log)))
	}
	manager.Register(trigger.New(cfg.Bot,
		trigger.WithLogger(log),
		trigger.WithAttention(attentionTracker),
	))
	manager.Register(admin.New(cfg.Bot.AdminIDs, mem, st, attentionTracker, admin.WithLogger(log)))

	toolRegistry := tools.New()
	tools.RegisterBuiltins(toolRegistry)

	styleLib := style.NewLibrary()
	manager.Register(stylelearn.New(llmClient, styleLib, stylelearn.WithLogger(log)))

	chatOpts := []chat.Option{
		chat.WithTools(toolRegistry),
		chat.WithMemory(mem),
		chat.WithLogger(log),
		chat.WithStyleLibrary(styleLib),
		chat.WithAttention(attentionTracker),
	}
	if st != nil {
		chatOpts = append(chatOpts, chat.WithStore(st))
	}
	if memeStore != nil {
		chatOpts = append(chatOpts, chat.WithMemes(memeStore))
	}
	manager.Register(chat.New(llmClient, personaPrompt, chatOpts...))

	limiter := ratelimit.New(5, 10)
	h := handler.New(client, manager, cfg.Bot.SelfID, log, handler.WithRateLimiter(limiter))

	client.SetMessageHandler(func(ctx context.Context, evt qq.Event) {
		h.Handle(ctx, evt)
	})

	if err := manager.Start(ctx, client); err != nil {
		log.Error("failed to start plugins", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := manager.Stop(context.Background()); err != nil {
			log.Error("failed to stop plugins", "error", err)
		}
	}()

	log.Info("connecting to OneBot", "url", cfg.QQ.OneBot.WSURL)
	if err := client.Connect(ctx); err != nil {
		log.Error("failed to connect OneBot", "error", err)
		os.Exit(1)
	}

	log.Info("bot started")
	<-ctx.Done()
	log.Info("shutting down")
}
