package cli

import (
	"context"

	rancherauth "github.com/rancher/rancher/pkg/auth"
	steveauth "github.com/rancher/steve/pkg/auth"
	authcli "github.com/rancher/steve/pkg/auth/cli"
	"github.com/rancher/steve/pkg/server"
	"github.com/rancher/wrangler/pkg/kubeconfig"
	"github.com/rancher/wrangler/pkg/ratelimit"
	"github.com/urfave/cli"
)

type Config struct {
	KubeConfig      string
	HTTPSListenPort int
	HTTPListenPort  int
	DashboardURL    string
	Authentication  bool

	WebhookConfig authcli.WebhookConfig
}

func (c *Config) MustServer(ctx context.Context) *server.Server {
	cc, err := c.ToServer(ctx)
	if err != nil {
		panic(err)
	}
	return cc
}

func (c *Config) ToServer(ctx context.Context) (*server.Server, error) {
	var (
		auth       steveauth.Middleware
		startHooks []server.StartHook
	)

	restConfig, err := kubeconfig.GetNonInteractiveClientConfig(c.KubeConfig).ClientConfig()
	if err != nil {
		return nil, err
	}
	restConfig.RateLimiter = ratelimit.None

	if c.Authentication {
		auth, err = c.WebhookConfig.WebhookMiddleware()
		if err != nil {
			return nil, err
		}

		if auth == nil {
			authServer, err := rancherauth.NewServer(ctx, restConfig)
			if err != nil {
				return nil, err
			}

			auth = authServer.Authenticator
			startHooks = append(startHooks, func(ctx context.Context, s *server.Server) error {
				s.Next = authServer.Management.Wrap(s.Next)
				return authServer.Start(ctx)
			})
		}
	}

	return &server.Server{
		RestConfig:     restConfig,
		AuthMiddleware: auth,
		DashboardURL: func() string {
			return c.DashboardURL
		},
		StartHooks: startHooks,
	}, nil
}

func Flags(config *Config) []cli.Flag {
	flags := []cli.Flag{
		cli.StringFlag{
			Name:        "kubeconfig",
			EnvVar:      "KUBECONFIG",
			Destination: &config.KubeConfig,
		},
		cli.IntFlag{
			Name:        "https-listen-port",
			Value:       9443,
			Destination: &config.HTTPSListenPort,
		},
		cli.IntFlag{
			Name:        "http-listen-port",
			Value:       9080,
			Destination: &config.HTTPListenPort,
		},
		cli.StringFlag{
			Name:        "dashboard-url",
			Value:       "https://releases.rancher.com/dashboard/latest/index.html",
			Destination: &config.DashboardURL,
		},
		cli.BoolTFlag{
			Name:        "authentication",
			Destination: &config.Authentication,
		},
	}

	return append(flags, authcli.Flags(&config.WebhookConfig)...)
}
