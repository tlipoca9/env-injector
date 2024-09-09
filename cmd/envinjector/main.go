package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/cockroachdb/errors"
	"github.com/tlipoca9/yevna"
	"github.com/tlipoca9/yevna/parser"
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

			logfile, err := os.OpenFile("/var/log/envinjector", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
			if err != nil {
				return errors.Wrapf(err, "cannot open log file")
			}

			logger := slog.NewTextHandler(logfile, &slog.HandlerOptions{
				Level: slog.LevelInfo,
			})
			if c.Bool("debug") {
				logger = slog.NewTextHandler(logfile, &slog.HandlerOptions{
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

/*
Examples:
[

	{
	  "binding": "envinjector",
	  "objects": [
	    {
	      "filterResult": {
	        "containers": [
	          {
	            "env": [
	              {
	                "name": "K8S_NODE_NAME",
	                "valueFrom": {
	                  "fieldRef": {
	                    "apiVersion": "v1",
	                    "fieldPath": "spec.nodeName"
	                  }
	                }
	              },
	              {
	                "name": "CILIUM_K8S_NAMESPACE",
	                "valueFrom": {
	                  "fieldRef": {
	                    "apiVersion": "v1",
	                    "fieldPath": "metadata.namespace"
	                  }
	                }
	              }
	            ],
	            "name": "cilium-envoy"
	          }
	        ],
	        "labels": {
	          "app.kubernetes.io/name": "cilium-envoy",
	          "app.kubernetes.io/part-of": "cilium",
	          "controller-revision-hash": "5df6b5d4db",
	          "k8s-app": "cilium-envoy",
	          "name": "cilium-envoy",
	          "pod-template-generation": "1"
	        },
	        "name": "cilium-envoy-9gx5c",
	        "namespace": "kube-system"
	      }
	    }
	  ],
	  "type": "Synchronization"
	}

]
*/
func hook(ctx context.Context, bindingContextPath string) error {
	var pods []Pod
	err := yevna.Run(
		ctx,
		yevna.OpenFile(bindingContextPath),
		yevna.Gjson("#.objects.#.filterResult|@flatten"),
		yevna.Unmarshal(parser.JSON(), &pods),
	)
	if err != nil {
		return err
	}
	slog.InfoContext(ctx, "received events", "pods_count", len(pods))
	for _, pod := range pods {
		log := slog.With("namespace", pod.Namespace, "name", pod.Name)
		for _, container := range pod.Containers {
			log = log.With("container", container.Name)
			log.InfoContext(ctx, "processing container")
			// TODO: check if container has env vars
		}
	}

	return nil
}
