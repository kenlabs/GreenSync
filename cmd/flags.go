package main

import "github.com/urfave/cli/v2"

var daemonFlags = []cli.Flag{
	&cli.StringFlag{
		Name:     "log-level",
		Usage:    "Set the log level",
		EnvVars:  []string{"GOLOG_LOG_LEVEL"},
		Value:    "debug",
		Required: false,
	},
}

var initFlags = []cli.Flag{}
