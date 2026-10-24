// Package app 负责 Icarosu 的依赖组装与生命周期管理。
package app

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"icarosu/internal/attention"
	"icarosu/internal/config"
	"icarosu/internal/dryrun"
	"icarosu/internal/group"
	"icarosu/internal/handler"
	"icarosu/internal/llm"
	"icarosu/internal/logger"
	"icarosu/internal/meme"
	"icarosu/internal/memory"
	"icarosu/internal/obs"
	"icarosu/internal/persona"
	"icarosu/internal/platform"
	"icarosu/internal/plugin"
	"icarosu/internal/plugin/admin"
	"icarosu/internal/plugin/builtin"
	"icarosu/internal/plugin/chat"
	"icarosu/internal/plugin/filter"
	"icarosu/internal/plugin/jargonlearn"
	"icarosu/internal/plugin/membersync"
	"icarosu/internal/plugin/recorder"
	"icarosu/internal/plugin/safety"
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

// App 持有机器人运行所需的全部依赖。
type App struct {
	cfgPath string
	cfgMgr  *config.Manager
	log     *slog.Logger
	logBuf  *logger.Buffer
	metrics *obs.Metrics
	scope   *group.Resolver

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	clientOverride platform.Adapter

	llmClient        *llm.Client
	st               *store.Store
	mem              *memory.Memory
	semanticSvc      *semantic.Service
	topicTracker     *topic.Tracker
	attentionTracker *attention.Tracker
	memeStore        *meme.Store
	visionSvc        *vision.Service
	personaPrompt    string

	client        platform.Adapter
	qqClient      *qq.Client
	botForPlugins plugin.Bot
	manager       *plugin.Manager

	toolRegistry *tools.Registry
	styleLib     *style.Library

	filterPlugin  *filter.FilterPlugin
	triggerPlugin *trigger.TriggerPlugin
	chatPlugin    *chat.ChatPlugin
	limiter       *ratelimit.Limiter

	sessionMgr *session.Manager
	handler    *handler.Handler
	eventPool  *queue.Pool
	webServer  *webui.Server

	summarizerPlugin *summarizer.SummarizerPlugin
}

// New 使用默认 OneBot 平台加载配置并组装所有依赖。
func New(cfgPath string) (*App, error) {
	return newApp(cfgPath, nil)
}

// NewWithPlatform 使用自定义平台适配器加载配置并组装依赖，便于测试和扩展。
func NewWithPlatform(cfgPath string, adapter platform.Adapter) (*App, error) {
	return newApp(cfgPath, adapter)
}

func newApp(cfgPath string, clientOverride platform.Adapter) (*App, error) {
	cfgMgr, err := config.NewManager(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	cfg := cfgMgr.Get()

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	log, logBuf := logger.NewWithBuffer(cfg.Log.Level, 1000)

	a := &App{
		cfgPath:        cfgPath,
		cfgMgr:         cfgMgr,
		log:            log,
		logBuf:         logBuf,
		metrics:        obs.NewMetrics(),
		clientOverride: clientOverride,
	}
	a.ctx, a.cancel = context.WithCancel(context.Background())
	builtin.SetSenderMetrics(a.metrics)
	a.scope = group.NewResolver(cfg.MemoryShareGroups, cfg.ExpressionShareGroups)

	// LLM 客户端
	a.llmClient = llm.NewClient(llm.Config{
		BaseURL:        cfg.LLM.BaseURL,
		APIKey:         cfg.LLM.APIKey,
		Model:          cfg.LLM.Model,
		Temperature:    cfg.LLM.Temperature,
		MaxTokens:      cfg.LLM.MaxTokens,
		Timeout:        time.Duration(cfg.LLM.TimeoutSeconds) * time.Second,
		CachePrefix:    cfg.LLM.CachePrefix,
		CacheMaxTokens: cfg.LLM.CacheMaxTokens,
		LogCache:       cfg.LLM.LogCache,
	}, log, llm.WithMetrics(a.metrics))
	log.Info("llm configured",
		"config_file", cfg.LLMConfigFile,
		"provider", cfg.LLM.Provider,
		"base_url", cfg.LLM.BaseURL,
		"model", cfg.LLM.Model,
		"api_key_set", cfg.LLM.APIKey != "",
	)

	// 长期存储与记忆服务
	if cfg.Memory.LongTermEnabled {
		a.st, err = store.Open(cfg.Store)
		if err != nil {
			return nil, fmt.Errorf("open store: %w", err)
		}
	}
	a.mem = memory.New(cfg.Memory.ShortTermSize, a.st)
	a.mem.SetContextMaxAge(cfg.Memory.ContextMaxAgeMinutes)
	a.mem.PreloadFromStore(cfg.Memory.ShortTermSize)

	if a.st != nil {
		a.semanticSvc = semantic.NewService(a.st, semantic.NewLocalEmbedder(256), log)
	}
	topicOpts := []topic.Option{}
	if a.st != nil {
		topicOpts = append(topicOpts, topic.WithStore(a.st))
	}
	a.topicTracker = topic.NewTracker(semantic.NewLocalEmbedder(256), 0, topicOpts...)
	a.attentionTracker = attention.New(cfg.Attention)

	// VLM + 表情包链路
	if cfg.VLM.BaseURL != "" && cfg.VLM.Model != "" {
		vlmClient := vlm.NewClient(vlm.Config{
			BaseURL: cfg.VLM.BaseURL,
			APIKey:  cfg.VLM.APIKey,
			Model:   cfg.VLM.Model,
			Prompt:  cfg.VLM.Prompt,
			Timeout: time.Duration(cfg.VLM.TimeoutSeconds) * time.Second,
		}, log)

		if err := vlmClient.Ping(context.Background()); err != nil {
			log.Warn("vlm not available, image chain disabled", "error", err)
		} else {
			a.memeStore, err = meme.Open(cfg.Meme.Directory)
			if err != nil {
				return nil, fmt.Errorf("open meme store: %w", err)
			}
			a.visionSvc = vision.New(vlmClient, a.memeStore, cfg.Meme)
			log.Info("vlm image understanding enabled", "model", cfg.VLM.Model)
		}
	}

	a.personaPrompt = persona.BuildSystemPrompt(cfg.Bot)

	client := a.clientOverride
	if client == nil {
		client = qq.NewClient(cfg.QQ.OneBot.WSURL, cfg.QQ.OneBot.AccessToken, log)
	}
	a.client = client
	if qc, ok := client.(*qq.Client); ok {
		a.qqClient = qc
	}
	a.botForPlugins = plugin.Bot(client)
	if cfg.DryRun {
		a.botForPlugins = dryrun.New(client, log)
		log.Warn("dry run mode enabled: outgoing messages will be logged but not sent")
	}

	// 插件管理器
	a.manager = plugin.NewManager()
	for _, p := range plugin.GlobalPlugins() {
		a.manager.Register(p)
	}

	a.filterPlugin = filter.New(cfg.Filter, filter.WithLogger(log))
	a.manager.Register(a.filterPlugin)
	a.manager.Register(safety.New(cfg.Bot, safety.WithLogger(log)))

	recorderOpts := []recorder.Option{recorder.WithLogger(log), recorder.WithTopicTracker(a.topicTracker)}
	if a.st != nil {
		recorderOpts = append(recorderOpts, recorder.WithStore(a.st))
	}
	if a.visionSvc != nil {
		recorderOpts = append(recorderOpts, recorder.WithVision(a.visionSvc))
	}
	a.manager.Register(recorder.New(a.mem, recorderOpts...))

	if a.st != nil && a.qqClient != nil {
		a.manager.Register(membersync.New(a.qqClient, a.st, membersync.WithLogger(log)))
	}
	if a.memeStore != nil && a.visionSvc != nil {
		a.manager.Register(steal.New(cfg.Meme, a.visionSvc, a.personaPrompt, steal.WithLogger(log)))
	}

	if a.st != nil && a.llmClient != nil {
		summarizerOpts := []summarizer.Option{summarizer.WithLogger(log)}
		if a.semanticSvc != nil {
			summarizerOpts = append(summarizerOpts, summarizer.WithSemantic(a.semanticSvc))
		}
		a.summarizerPlugin = summarizer.New(a.llmClient, a.mem, a.st, cfg.Memory, summarizerOpts...)
		a.manager.Register(a.summarizerPlugin)
	}

	a.triggerPlugin = trigger.New(cfg.Bot,
		trigger.WithLogger(log),
		trigger.WithAttention(a.attentionTracker),
	)
	a.manager.Register(a.triggerPlugin)
	a.manager.Register(admin.New(cfg.Bot.AdminIDs, a.mem, a.st, a.attentionTracker, admin.WithLogger(log)))

	a.toolRegistry = tools.New()
	tools.RegisterBuiltins(a.toolRegistry)

	a.styleLib = style.NewLibrary()
	styleOpts := []stylelearn.Option{stylelearn.WithLogger(log), stylelearn.WithGroupScope(a.scope)}
	if a.st != nil {
		if records, err := a.st.LoadStyleExpressions(); err == nil {
			for _, r := range records {
				a.styleLib.AddForGroup(r.GroupID, r.Tags, r.Expression)
			}
		}
		styleOpts = append(styleOpts, stylelearn.WithSaver(a.st))
	}
	a.manager.Register(stylelearn.New(a.llmClient, a.styleLib, styleOpts...))

	jargonOpts := []jargonlearn.Option{jargonlearn.WithLogger(log), jargonlearn.WithGroupScope(a.scope)}
	if a.st != nil {
		jargonOpts = append(jargonOpts, jargonlearn.WithStore(a.st))
	}
	a.manager.Register(jargonlearn.New(a.llmClient, jargonOpts...))

	chatOpts := []chat.Option{
		chat.WithTools(a.toolRegistry),
		chat.WithMemory(a.mem),
		chat.WithLogger(log),
		chat.WithStyleLibrary(a.styleLib),
		chat.WithAttention(a.attentionTracker),
		chat.WithTopicTracker(a.topicTracker),
		chat.WithGroupScope(a.scope),
	}
	if a.st != nil {
		chatOpts = append(chatOpts,
			chat.WithStore(a.st),
			chat.WithJargons(a.st),
			chat.WithProfiles(a.st),
		)
	}
	if a.semanticSvc != nil {
		chatOpts = append(chatOpts, chat.WithSemantic(a.semanticSvc))
	}
	if a.memeStore != nil {
		chatOpts = append(chatOpts, chat.WithMemes(a.memeStore))
	}
	a.chatPlugin = chat.New(a.llmClient, a.personaPrompt, chatOpts...)
	a.manager.Register(a.chatPlugin)

	a.limiter = ratelimit.New(cfg.RateLimit.Rate, cfg.RateLimit.Burst)

	a.registerReload(cfg)

	a.sessionMgr = session.New(24 * time.Hour)
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.sessionMgr.CleanupLoop(a.ctx, time.Hour)
	}()
	a.handler = handler.New(a.botForPlugins, a.manager, cfg.Bot.SelfID, log,
		handler.WithRateLimiter(a.limiter),
		handler.WithSessionManager(a.sessionMgr),
		handler.WithMetrics(a.metrics),
	)

	a.eventPool = queue.NewPool(cfg.Queue.Workers, cfg.Queue.Buffer)

	if cfg.WebUI.Enabled {
		webuiOpts := []webui.Option{
			webui.WithLogBuffer(a.logBuf),
			webui.WithAttention(a.attentionTracker),
			webui.WithLLM(a.llmClient),
			webui.WithPluginManager(a.manager),
			webui.WithMemory(a.mem),
			webui.WithBot(a.botForPlugins),
		}
		if a.st != nil {
			webuiOpts = append(webuiOpts, webui.WithStore(a.st))
		}
		if a.memeStore != nil {
			webuiOpts = append(webuiOpts, webui.WithMemeStore(a.memeStore))
		}
		if a.summarizerPlugin != nil {
			webuiOpts = append(webuiOpts, webui.WithSummarizer(a.summarizerPlugin))
		}
		a.webServer = webui.New(a.cfgMgr, log, webuiOpts...)
	}

	return a, nil
}

func (a *App) registerReload(cfg *config.Config) {
	a.cfgMgr.OnReload(func(cfg *config.Config) {
		a.llmClient.UpdateConfig(llm.Config{
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
		a.filterPlugin.Update(cfg.Filter)
		a.triggerPlugin.Update(cfg.Bot)
		a.attentionTracker.Update(cfg.Attention)
		a.limiter.Update(cfg.RateLimit.Rate, cfg.RateLimit.Burst)
		a.chatPlugin.UpdatePersona(persona.BuildSystemPrompt(cfg.Bot))
		a.log.Info("config hot reloaded")
	})
}

// Run 启动 WebUI、插件、QQ 连接并等待退出。
func (a *App) Run(ctx context.Context) error {
	if a.webServer != nil {
		if err := a.webServer.Start(); err != nil {
			return fmt.Errorf("start webui: %w", err)
		}
		defer a.webServer.Close()
	}

	a.client.SetMessageHandler(func(ctx context.Context, evt qq.Event) {
		_ = a.eventPool.Submit(ctx, func(ctx context.Context) {
			a.handler.Handle(ctx, evt)
		})
	})

	if err := a.manager.Start(ctx, a.botForPlugins); err != nil {
		return fmt.Errorf("start plugins: %w", err)
	}
	defer func() {
		_ = a.manager.Stop(context.Background())
	}()
	defer a.eventPool.Close()

	a.log.Info("connecting to OneBot", "url", a.cfgMgr.Get().QQ.OneBot.WSURL)
	if err := a.client.Connect(ctx); err != nil {
		return fmt.Errorf("connect onebot: %w", err)
	}

	a.log.Info("bot started")
	<-ctx.Done()
	a.cancel()
	a.log.Info("shutting down")
	return nil
}

// Close 释放 App 持有的资源，供测试或手动退出使用。
func (a *App) Close() error {
	if a.cancel != nil {
		a.cancel()
	}
	a.wg.Wait()

	if a.webServer != nil {
		_ = a.webServer.Close()
	}
	if a.eventPool != nil {
		a.eventPool.Close()
	}
	if a.st != nil {
		return a.st.Close()
	}
	return nil
}
