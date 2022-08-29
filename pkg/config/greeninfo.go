package config

import (
	"fmt"
	"net/url"
	"time"
)

const (
	defaultGreenLocationUrl = "https://provider-quest.s3.us-west-2.amazonaws.com/dist/geoip-lookups/miner-locations-latest.json"
	defaultCheckInterval    = time.Minute
	defaultSaveLocationData = true
)

type GreenInfo struct {
	Url              string
	CheckInterval    string
	SaveLocationData bool
}

func NewGreenInfo() GreenInfo {
	return GreenInfo{
		Url:              defaultGreenLocationUrl,
		CheckInterval:    defaultCheckInterval.String(),
		SaveLocationData: defaultSaveLocationData,
	}
}

func (gi *GreenInfo) Validate() error {
	gUrl, err := url.Parse(gi.Url)
	if err != nil {
		return err
	}
	if gUrl.Scheme != "https" {
		return fmt.Errorf("only support https, got: %s", gUrl.Scheme)
	}
	//if gUrl.Host != "hub.textile.io" {
	//	return fmt.Errorf("wrong host name, expected: hub.textile.io, got: %s", gUrl.Host)
	//}

	_, err = time.ParseDuration(gi.CheckInterval)
	if err != nil {
		return err
	}
	return nil
}
