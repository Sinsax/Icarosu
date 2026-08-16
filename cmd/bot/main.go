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
	"icarosu/internal/dryrun"
	"icarosu/internal/handler"
	"icarosu/internal/llm"
	"icarosu/internal/logger"
	"icarosu/internal/meme"
	"icarosu/internal/memory"
	"icarosu/internal/persona"
	"icarosu/internal/platform"
	"icarosu/internal/platform/telegram"
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
	"icarosu/internal/queue"
	"icarosu/internal/ratelimit"
	"icarosu/internal/semantic"
	"icarosu/internal/session"
	"icarosu/internal/store"
	"icarosu/internal/style"
	"icarosu/internal/tools"
	"icarosu/internal/topic"
	"icarosu/internal/vision"
	"icarosu/internal/vlm"
	"icarosu/internal/webui"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "configs/config.yaml", "path to config file")
	flag.Parse()

	cfgMgr, err := config.NewManager(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}
	cfg := cfgMgr.Get()

	log, logBuf := logger.NewWithBuffer(cfg.Log.Level, 1000)

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

	var semanticSvc *semantic.Service
	if st != nil {
		semanticSvc = semantic.NewService(st, semantic.NewLocalEmbedder(256), log)
	}
	topicOpts := []topic.Option{}
	if st != nil {
		topicOpts = append(topicOpts, topic.WithStore(st))
	}
	topicTracker := topic.NewTracker(semantic.NewLocalEmbedder(256), 0, topicOpts...)

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

	var client platform.Adapter
	var qqClient *qq.Client
	if cfg.Telegram.Enabled {
		client = telegram.NewClient(cfg.Telegram.Token, log)
	} else {
		qqClient = qq.NewClient(cfg.QQ.OneBot.WSURL, cfg.QQ.OneBot.AccessToken, log)
		client = qqClient
	}

	botForPlugins := plugin.Bot(client)
	if cfg.DryRun {
		botForPlugins = dryrun.New(client, log)
		log.Warn("dry run mode enabled: outgoing messages will be logged but not sent")
	}

	// 插件管理器：后续所有扩展（AI 聊天、管理、娱乐等）都以插件形式注册到这里。
	// 简单插件可通过 init() 自注册到全局注册表；需要依赖的插件在 main 中显式构造注册。
	manager := plugin.NewManager()
	for _, p := range plugin.GlobalPlugins() {
		manager.Register(p)
	}
	filterPlugin := filter.New(cfg.Filter, filter.WithLogger(log))
	manager.Register(filterPlugin)
	recorderOpts := []recorder.Option{recorder.WithLogger(log), recorder.WithTopicTracker(topicTracker)}
	if st != nil {
		recorderOpts = append(recorderOpts, recorder.WithStore(st))
	}
	if visionSvc != nil {
		recorderOpts = append(recorderOpts, recorder.WithVision(visionSvc))
	}
	manager.Register(recorder.New(mem, recorderOpts...))
	if st != nil && qqClient != nil {
		manager.Register(membersync.New(qqClient, st, membersync.WithLogger(log)))
	}
	if memeStore != nil && visionSvc != nil {
		manager.Register(steal.New(cfg.Meme, visionSvc, personaPrompt, steal.WithLogger(log)))
	}
	var summarizerPlugin *summarizer.SummarizerPlugin
	if st != nil && llmClient != nil {
		summarizerOpts := []summarizer.Option{summarizer.WithLogger(log)}
		if semanticSvc != nil {
			summarizerOpts = append(summarizerOpts, summarizer.WithSemantic(semanticSvc))
		}
		summarizerPlugin = summarizer.New(llmClient, mem, st, cfg.Memory, summarizerOpts...)
		manager.Register(summarizerPlugin)
	}
	triggerPlugin := trigger.New(cfg.Bot,
		trigger.WithLogger(log),
		trigger.WithAttention(attentionTracker),
	)
	manager.Register(triggerPlugin)
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
		chat.WithTopicTracker(topicTracker),
	}
	if st != nil {
		chatOpts = append(chatOpts, chat.WithStore(st))
	}
	if semanticSvc != nil {
		chatOpts = append(chatOpts, chat.WithSemantic(semanticSvc))
	}
	if memeStore != nil {
		chatOpts = append(chatOpts, chat.WithMemes(memeStore))
	}
	chatPlugin := chat.New(llmClient, personaPrompt, chatOpts...)
	manager.Register(chatPlugin)

	limiter := ratelimit.New(cfg.RateLimit.Rate, cfg.RateLimit.Burst)

	cfgMgr.OnReload(func(cfg *config.Config) {
		llmClient.UpdateConfig(llm.Config{
			BaseURL:        cfg.LLM.BaseURL,
			APIKey:         cfg.LLM.APIKey,
			Model:          cfg.LLM.Model,
			Temperature:    cfg.LLM.Temperature,
			MaxTokens:      cfg.LLM.MaxTokens,
			Timeout:        time.Duration(cfg.LLM.TimeoutSeconds) * time.Second,
			CachePrefix:    cfg.LLM.CachePrefix,
			CacheMaxTokens: cfg.LLM.CacheMaxTokens,
			LogCache:       cfg.LLM.LogCache,
		})
		filterPlugin.Update(cfg.Filter)
		triggerPlugin.Update(cfg.Bot)
		attentionTracker.Update(cfg.Attention)
		limiter.Update(cfg.RateLimit.Rate, cfg.RateLimit.Burst)
		chatPlugin.UpdatePersona(persona.BuildSystemPrompt(cfg.Bot))
		log.Info("config hot reloaded")
	})

	sessionMgr := session.New(24 * time.Hour)
	go sessionMgr.CleanupLoop(ctx, time.Hour)
	h := handler.New(botForPlugins, manager, cfg.Bot.SelfID, log, handler.WithRateLimiter(limiter), handler.WithSessionManager(sessionMgr))

	eventPool := queue.NewPool(cfg.Queue.Workers, cfg.Queue.Buffer)
	if cfg.WebUI.Enabled {
		webuiOpts := []webui.Option{webui.WithLogBuffer(logBuf), webui.WithAttention(attentionTracker), webui.WithLLM(llmClient), webui.WithPluginManager(manager), webui.WithMemory(mem), webui.WithBot(botForPlugins)}
		if st != nil {
			webuiOpts = append(webuiOpts, webui.WithStore(st))
		}
		if memeStore != nil {
			webuiOpts = append(webuiOpts, webui.WithMemeStore(memeStore))
		}
		if summarizerPlugin != nil {
			webuiOpts = append(webuiOpts, webui.WithSummarizer(summarizerPlugin))
		}
		webServer := webui.New(cfgMgr, log, webuiOpts...)
		if err := webServer.Start(); err != nil {
			log.Error("failed to start webui", "error", err)
			os.Exit(1)
		}
		defer webServer.Close()
	}

	client.SetMessageHandler(func(ctx context.Context, evt qq.Event) {
		_ = eventPool.Submit(ctx, func(ctx context.Context) {
			h.Handle(ctx, evt)
		})
	})

	if err := manager.Start(ctx, botForPlugins); err != nil {
		log.Error("failed to start plugins", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := manager.Stop(context.Background()); err != nil {
			log.Error("failed to stop plugins", "error", err)
		}
	}()
	defer eventPool.Close()

	if cfg.Telegram.Enabled {
		log.Info("connecting to Telegram")
	} else {
		log.Info("connecting to OneBot", "url", cfg.QQ.OneBot.WSURL)
	}
	if err := client.Connect(ctx); err != nil {
		log.Error("failed to connect platform", "error", err)
		os.Exit(1)
	}

	log.Info("bot started")
	<-ctx.Done()
	log.Info("shutting down")
}
