package project

import (
	"context"
	"time"
)

type Project struct {
	ID                     string     `json:"id"`
	UserID                 string     `json:"userId"`
	ExternalProjectID      string     `json:"externalProjectId"`
	TenantID               string     `json:"tenantId"`
	Slug                   string     `json:"slug"`
	Name                   string     `json:"name"`
	Description            string     `json:"description"`
	Status                 string     `json:"status"`
	Visibility             string     `json:"visibility"`
	WebsiteTrackingEnabled bool       `json:"websiteTrackingEnabled"`
	WebsiteDomains         []string   `json:"websiteDomains"`
	PublicTokenHash        string     `json:"publicTokenHash"`
	DefaultTheme           string     `json:"defaultTheme"`
	DefaultLocale          string     `json:"defaultLocale"`
	RenderEnabled          bool       `json:"renderEnabled"`
	BadgeEnabled           bool       `json:"badgeEnabled"`
	WidgetEnabled          bool       `json:"widgetEnabled"`
	ChartEnabled           bool       `json:"chartEnabled"`
	LastSyncedAt           *time.Time `json:"lastSyncedAt"`
	DeletedAt              *time.Time `json:"deletedAt"`
	CreatedAt              time.Time  `json:"createdAt"`
	UpdatedAt              time.Time  `json:"updatedAt"`
}

type ProjectLimits struct {
	ID                   string
	ProjectID            string
	PlanCode             string
	MaxRequestsPerMinute *int
	CacheTTLSeconds      *int
	BadgeEnabled         bool
	WidgetEnabled        bool
	ChartEnabled         bool
	RenderEnabled        bool
	EffectiveFrom        *time.Time
	EffectiveUntil       *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type WidgetSettings struct {
	ID        string
	ProjectID string
	WidgetKey string
	Status    string
	Settings  map[string]interface{}
	Version   int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ProjectCounter struct {
	ProjectID   string
	CounterName string
	Value       int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Repository interface {
	GetByID(ctx context.Context, id string) (*Project, error)
	GetByIDAndUser(ctx context.Context, id, userID string) (*Project, error)
	ListByUser(ctx context.Context, userID string) ([]*Project, error)
	ListAll(ctx context.Context) ([]*Project, error)
	GetBySlug(ctx context.Context, slug string) (*Project, error)
	GetByExternalID(ctx context.Context, externalID string) (*Project, error)
	Create(ctx context.Context, project *Project) error
	Update(ctx context.Context, project *Project) error
	Delete(ctx context.Context, id, userID string) error
	Upsert(ctx context.Context, project *Project) error

	GetLimits(ctx context.Context, projectID string) (*ProjectLimits, error)
	UpsertLimits(ctx context.Context, limits *ProjectLimits) error

	GetWidgetSettings(ctx context.Context, projectID, widgetKey string) (*WidgetSettings, error)
	UpsertWidgetSettings(ctx context.Context, settings *WidgetSettings) error

	GetCounter(ctx context.Context, projectID, counterName string) (int64, error)
	UpsertCounter(ctx context.Context, projectID, counterName string, value int64) error
	ListCounters(ctx context.Context, projectID string) (map[string]int64, error)
}
