package dd

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
)

const DefaultSite = "datadoghq.com"

type Options struct {
	Site        string
	APIKey      string
	AppKey      string
	AccessToken string
	Debug       bool
	Timeout     time.Duration
}

type Client struct {
	Site      string
	API       *datadog.APIClient
	Context   context.Context
	UsingAuth string
}

func ResolveOptions(opts Options) Options {
	if opts.Site == "" {
		opts.Site = firstNonEmpty(os.Getenv("DD_SITE"), DefaultSite)
	}
	if opts.APIKey == "" {
		opts.APIKey = os.Getenv("DD_API_KEY")
	}
	if opts.AppKey == "" {
		opts.AppKey = firstNonEmpty(os.Getenv("DD_APP_KEY"), os.Getenv("DD_APPLICATION_KEY"))
	}
	if opts.AccessToken == "" {
		opts.AccessToken = os.Getenv("DD_ACCESS_TOKEN")
	}
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}
	opts.Site = normalizeSite(opts.Site)
	return opts
}

func NewClient(ctx context.Context, opts Options) (*Client, error) {
	opts = ResolveOptions(opts)
	if err := validateAuth(opts); err != nil {
		return nil, err
	}

	cfg := datadog.NewConfiguration()
	cfg.Debug = opts.Debug
	cfg.RetryConfiguration.EnableRetry = true
	cfg.HTTPClient = &http.Client{Timeout: opts.Timeout}

	api := datadog.NewAPIClient(cfg)
	ctx = context.WithValue(ctx, datadog.ContextServerVariables, map[string]string{"site": opts.Site})
	authKind := "api_app_key"
	if opts.AccessToken != "" {
		authKind = "access_token"
		ctx = context.WithValue(ctx, datadog.ContextAccessToken, opts.AccessToken)
	} else {
		ctx = context.WithValue(ctx, datadog.ContextAPIKeys, map[string]datadog.APIKey{
			"apiKeyAuth": {Key: opts.APIKey},
			"appKeyAuth": {Key: opts.AppKey},
		})
	}

	return &Client{
		Site:      opts.Site,
		API:       api,
		Context:   ctx,
		UsingAuth: authKind,
	}, nil
}

func validateAuth(opts Options) error {
	if opts.AccessToken != "" {
		return nil
	}
	if opts.APIKey == "" && opts.AppKey == "" {
		return errors.New("missing Datadog credentials: set DD_API_KEY and DD_APP_KEY, pass --api-key and --app-key, or set DD_ACCESS_TOKEN")
	}
	if opts.APIKey == "" {
		return errors.New("missing Datadog API key: set DD_API_KEY or pass --api-key")
	}
	if opts.AppKey == "" {
		return errors.New("missing Datadog application key: set DD_APP_KEY, DD_APPLICATION_KEY, or pass --app-key")
	}
	return nil
}

func normalizeSite(site string) string {
	site = strings.TrimSpace(site)
	site = strings.TrimPrefix(site, "https://")
	site = strings.TrimPrefix(site, "http://")
	site = strings.TrimPrefix(site, "app.")
	site = strings.TrimPrefix(site, "api.")
	site = strings.TrimRight(site, "/")
	if site == "" {
		return DefaultSite
	}
	return site
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
