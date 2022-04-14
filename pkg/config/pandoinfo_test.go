package config

import (
	"encoding/json"
	"net/http"
	"testing"
)

type Res struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		PeerID       string `json:"peerID"`
		APIAddresses struct {
			GRAPHQL_API   string `json:"GRAPHQL_API"`
			GRAPHSYNC_API string `json:"GRAPHSYNC_API"`
			HTTP_API      string `json:"HTTP_API"`
		} `json:"APIAddresses"`
	} `json:"Data"`
}

func TestGetPandoInfoFromKenLabs(t *testing.T) {
	res, err := http.Get("https://pando-api.kencloud.com/pando/info")
	if err != nil {
		t.Fatal(err.Error())
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("wrong http code: %v", res.StatusCode)
	}
	pinfo := new(Res)
	err = json.NewDecoder(res.Body).Decode(&pinfo)
	if err != nil {
		t.Fatalf(err.Error())
	}
	t.Log(pinfo)

}
