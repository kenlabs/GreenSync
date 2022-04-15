package main

import (
	"GreenSync/pkg/config"
	"github.com/urfave/cli/v2"
)

var InitCmd = &cli.Command{
	Name:   "init",
	Usage:  "Initialize GreenSync config file and identity",
	Flags:  initFlags,
	Action: initCommand,
}

func initCommand(cctx *cli.Context) error {
	log.Info("Initializing provider config file")

	// Check that the config root exists and it writable.
	configRoot, err := config.PathRoot()
	if err != nil {
		return err
	}

	if err = checkWritable(configRoot); err != nil {
		return err
	}

	configFile, err := config.Filename(configRoot)
	if err != nil {
		return err
	}

	if fileExists(configFile) {
		return config.ErrInitialized
	}

	cfg, err := config.Init(cctx.App.Writer)
	if err != nil {
		return err
	}

	// Use values from flags to override defaults
	// cfg.Identity = struct{}{}

	return cfg.Save(configFile)
}
