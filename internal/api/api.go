package api

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/svgstat/svgstat/internal/admin"
	"github.com/svgstat/svgstat/internal/analytics"
	"github.com/svgstat/svgstat/internal/auth"
	"github.com/svgstat/svgstat/internal/cache"
	"github.com/svgstat/svgstat/internal/config"
	"github.com/svgstat/svgstat/internal/conversion"
	"github.com/svgstat/svgstat/internal/counter"
	"github.com/svgstat/svgstat/internal/database"
	"github.com/svgstat/svgstat/internal/diagnostics"
	"github.com/svgstat/svgstat/internal/geoip"
	"github.com/svgstat/svgstat/internal/metrics"
	"github.com/svgstat/svgstat/internal/observability"
	"github.com/svgstat/svgstat/internal/project"
	"github.com/svgstat/svgstat/internal/renderer"
	"github.com/svgstat/svgstat/internal/requestmeta"
	"github.com/svgstat/svgstat/internal/worker"
)

type App struct {
	config         *config.Config
	db             *database.Database
	cache          *cache.Cache
	auth           *auth.Manager
	admin          *admin.Service
	analytics      *analytics.Analytics
	counter        *counter.Counter
	renderer       *renderer.Renderer
	projectRepo    project.Repository
	runtime        *project.RuntimeCache
	metrics        *metrics.Service
	conversion     *conversion.Service
	diagnostics    *diagnostics.Service
	geoIP          *geoip.GeoIP
	worker         *worker.Worker
	observability  *observability.Metrics
	requestMeta    *requestmeta.Resolver
	authLimiter    *rateLimiter
	collectLimiter *rateLimiter
	runtimeCancel  context.CancelFunc
}

func NewApp(cfg *config.Config) (*App, error) {
	db, err := database.New(cfg)
	if err != nil {
		return nil, err
	}

	c, err := cache.New(cfg)
	if err != nil {
		return nil, err
	}

	var g *geoip.GeoIP
	if cfg.GeoIP.DBPath != "" {
		g, err = geoip.New(cfg.GeoIP.DBPath)
		if err != nil {
			log.Warn().Err(err).Str("path", cfg.GeoIP.DBPath).Msg("Failed to load GeoIP database, geolocation will be disabled")
		}
	}

	projectRepo := project.NewPostgresRepository(db.Pool)
	requestResolver := requestmeta.NewResolver(cfg.HTTP.TrustedProxies)
	runtimeCache := project.NewRuntimeCache(projectRepo, c, cfg.Cache.TTL)
	telemetry := observability.New()
	runtimeCache.SetObserver(telemetry)
	runtimeCtx, runtimeCancel := context.WithCancel(context.Background())
	runtimeCache.StartInvalidationSubscriber(runtimeCtx)
	if err := runtimeCache.Warm(context.Background()); err != nil {
		log.Warn().Err(err).Msg("Failed to warm runtime project cache")
	}
	analyticsSvc := analytics.New(c, projectRepo, g, cfg.Analytics.KeyTTL, cfg.Analytics.IPSalt)
	analyticsSvc.SetCardinalityLimits(cfg.Analytics.MaxDailyVisitors, cfg.Analytics.MaxDimensionValues, telemetry)
	analyticsSvc.SetClientIPResolver(requestResolver)
	conversionSvc := conversion.New(db.Pool, c)
	if err := conversionSvc.Warm(context.Background()); err != nil {
		log.Warn().Err(err).Msg("Failed to warm conversion funnel cache")
	}

	authManager := auth.NewManager(db.Pool, cfg.Admin.Emails)
	if err := authManager.BootstrapAdmins(context.Background()); err != nil {
		log.Warn().Err(err).Msg("Failed to bootstrap configured administrators")
	}

	workerSvc := worker.New(analyticsSvc, projectRepo, db.Pool, cfg.Worker.FlushInterval)
	workerSvc.SetLeadership(cfg.Worker.LockID, telemetry)
	app := &App{
		config:         cfg,
		db:             db,
		cache:          c,
		auth:           authManager,
		admin:          admin.NewService(db.Pool, c, runtimeCache),
		analytics:      analyticsSvc,
		counter:        counter.New(c, projectRepo, cfg.Analytics.KeyTTL),
		renderer:       renderer.New(),
		projectRepo:    projectRepo,
		runtime:        runtimeCache,
		metrics:        metrics.New(db.Pool, analyticsSvc),
		conversion:     conversionSvc,
		diagnostics:    diagnostics.New(c),
		geoIP:          g,
		worker:         workerSvc,
		observability:  telemetry,
		requestMeta:    requestResolver,
		authLimiter:    newRateLimiter(10, 15*time.Minute, "auth", c),
		collectLimiter: newRateLimiter(120, time.Minute, "collect", c),
		runtimeCancel:  runtimeCancel,
	}

	return app, nil
}

// StartWorker runs the analytics flush worker until ctx is cancelled.
func (a *App) StartWorker(ctx context.Context) {
	if !a.config.Worker.Enabled {
		log.Info().Msg("Analytics worker disabled")
		return
	}
	a.worker.Start(ctx)
}

func (a *App) Close() {
	if a.runtimeCancel != nil {
		a.runtimeCancel()
	}
	a.db.Close()
	_ = a.cache.Close()
	if a.geoIP != nil {
		_ = a.geoIP.Close()
	}
}
