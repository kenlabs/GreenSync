package config

import (
	"time"
)

const (
	defaultCrawlInterval     = time.Minute
	defaultSyncCheckInterval = time.Hour
	defaultCleanAfterSync    = false
)

type IngestCfg struct {
	CrawlInterval     string
	SyncCheckInterval string
	CleanAfterSync    bool
}

func NewIngestCfg() IngestCfg {
	return IngestCfg{
		CrawlInterval:     defaultCrawlInterval.String(),
		SyncCheckInterval: defaultSyncCheckInterval.String(),
		CleanAfterSync:    defaultCleanAfterSync,
	}
}

func (ic *IngestCfg) Validate() error {
	_, err := time.ParseDuration(ic.CrawlInterval)
	if err != nil {
		return err
	}
	_, err = time.ParseDuration(ic.SyncCheckInterval)
	if err != nil {
		return err
	}
	return nil
}
