package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/tlipoca9/yevna"
	"github.com/urfave/cli/v2"
)

const CONFIG = `
configVersion: v1
kubernetes:
- apiVersion: v1
  kind: Pod
  name: envinjector
  executeHookOnEvent: [ "Added", "Modified" ]
  jqFilter: |
    {
      "namespace": .metadata.namespace,
      "name": .metadata.name,
      "labels": .metadata.labels,
      "containers": [.spec.containers[] | {name: .name, env: .env}],
    }
`

type Pod struct {
	Namespace  string            `json:"namespace"`
	Name       string            `json:"name"`
	Labels     map[string]string `json:"labels"`
	Containers []Container       `json:"containers"`
}

type Container struct {
	Name string   `json:"name"`
	Env  []EnvVar `json:"env"`
}

type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func main() {
	app := &cli.App{
		Name:  "envinjector",
		Usage: "envinjector is a tool to inject environment variables into pods, it depends on shell-operator",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name: "config",
			},
			&cli.PathFlag{
				Name:    "binding-context-path",
				EnvVars: []string{"BINDING_CONTEXT_PATH"},
			},
			&cli.BoolFlag{
				Name:    "debug",
				EnvVars: []string{"ENVINJECTOR_DEBUG"},
			},
			&cli.BoolFlag{
				Name:    "overwrite",
				Usage:   "Overwrite existing environment variables",
				EnvVars: []string{"ENVINJECTOR_OVERWRITE"},
			},
		},
		Action: func(c *cli.Context) error {
			if c.Bool("config") {
				fmt.Print(CONFIG[1:])
				return nil
			}

			logger := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: slog.LevelInfo,
			})
			if c.Bool("debug") {
				logger = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
					Level: slog.LevelDebug,
				})
			}
			slog.SetDefault(slog.New(logger))
			return hook(c.Context, c.Path("binding-context-path"))
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func hook(ctx context.Context, bindingContextPath string) error {
	var bindingContextPathStr string
	err := yevna.Run(
		ctx,
		yevna.OpenFile(bindingContextPath),
		yevna.ToStr(),
		yevna.Output(&bindingContextPathStr),
		// yevna.Gjson(".[].object"),
		// yevna.Unmarshal(parser.JSON(), &pods),
	)
	if err != nil {
		return err
	}
	slog.InfoContext(ctx, "received events", "events", bindingContextPathStr)
	// var pods []Pod
	// slog.InfoContext(ctx, "received events", "pods_count", len(pods))
	// for _, pod := range pods {
	// 	log := slog.With("namespace", pod.Namespace, "name", pod.Name)
	// 	for _, container := range pod.Containers {
	// 		log = log.With("container", container.Name)
	// 		log.DebugContext(ctx, "processing container")
	// 		// TODO: check if container has env vars
	// 	}
	// }

	return nil
}
