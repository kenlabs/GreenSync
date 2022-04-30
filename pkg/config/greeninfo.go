package config

import (
	"fmt"
	"net/url"
)

const (
	defaultGreenLocationUrl = "https://hub.textile.io/thread/bafkwblbznyqkmqx5l677z3kjsslhxo2vbbqh6wluunvvdbmqattrdya/buckets/bafzbeibjg7kky45npdwnogui5ffla7dint62xpttvvlzrsbewlrfmbusya/miner-locations-latest.json"
)

type GreenInfo struct {
	Url string
}

func NewGreenInfo() GreenInfo {
	return GreenInfo{
		Url: defaultGreenLocationUrl,
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
	if gUrl.Host != "hub.textile.io" {
		return fmt.Errorf("wrong host name, expected: hub.textile.io, got: %s", gUrl.Host)
	}
	return nil
}
