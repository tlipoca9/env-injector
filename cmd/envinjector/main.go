package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/cockroachdb/errors"
	"github.com/oklog/ulid/v2"
	"github.com/tlipoca9/env-injector/operation"
	"github.com/tlipoca9/yevna"
	"github.com/tlipoca9/yevna/parser"
	"github.com/urfave/cli/v2"
)

type CtxKey string

const (
	CONFIG = `
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
	EventIDCtxKey CtxKey = "event_id"
)

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
			&cli.PathFlag{
				Name:    "k8s-patch-path",
				EnvVars: []string{"KUBERNETES_PATCH_PATH"},
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
			ctx := context.WithValue(c.Context, EventIDCtxKey, ulid.Make().String())
			return hook(ctx, c.Path("binding-context-path"), c.Path("k8s-patch-path"))
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
func hook(ctx context.Context, bindingContextPath string, k8sPatchPath string) error {
	log := slog.With("event_id", ctx.Value(EventIDCtxKey))
	log.InfoContext(ctx, "received event", "binding-context-path", bindingContextPath, "k8s-patch-path", k8sPatchPath)

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
	log.InfoContext(ctx, "event parse success", "pods_count", len(pods))

	tmpl := `.spec.containers[%d].env = .spec.containers[%d].env + %s`
	operations := make([]operation.JQPatch, 0)
	for _, pod := range pods {
		log := log.With("namespace", pod.Namespace, "name", pod.Name)
		for i, container := range pod.Containers {
			log = log.With("container", container.Name)
			log.InfoContext(ctx, "processing container")
			needAddEnvs := map[string]bool{
				"TEST_ENVINJECTOR": true,
			}
			for _, envVar := range container.Env {
				if needAddEnvs[envVar.Name] {
					needAddEnvs[envVar.Name] = false
				}
			}
			envVars := make([]EnvVar, 0)
			for k, v := range needAddEnvs {
				if !v {
					continue
				}
				envVars = append(envVars, EnvVar{
					Name:  k,
					Value: k,
				})
			}
			buf, err := json.Marshal(envVars)
			if err != nil {
				return err
			}
			ope := operation.JQPatch{
				APIVersion: "v1",
				Kind:       "Pod",
				Namespace:  pod.Namespace,
				Name:       pod.Name,
				JQFilter:   fmt.Sprintf(tmpl, i, i, string(buf)),
			}
			operations = append(operations, ope)
		}
	}

	return yevna.Run(
		ctx,
		yevna.Input(operations),
		yevna.HandlerFunc(func(c *yevna.Context, in any) (any, error) {
			return json.Marshal(in)
		}),
		yevna.WriteFile(k8sPatchPath),
	)
}
