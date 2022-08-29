package config

import (
	"encoding/json"
	"fmt"
	"github.com/libp2p/go-libp2p-core/peer"
	"net/http"
)

const (
	kenlbasInfoUrl = "https://pando-api.kencloud.com/pando/info"
	pandoAPIUrl    = "https://pando-api.kencloud.com"
)

type httpRes struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		PeerID       string `json:"peerID"`
		APIAddresses struct {
			GRAPHQL_API   string `json:"GraphQLAPI"`
			GRAPHSYNC_API string `json:"GraphSyncAPI"`
			HTTP_API      string `json:"HttpAPI"`
		} `json:"Addresses"`
	} `json:"Data"`
}

type PandoInfo struct {
	PandoMultiAddr string
	PandoPeerID    string
	PandoAPIUrl    string
	TopicName      string
}

func (pinfo *PandoInfo) AddrInfo() (*peer.AddrInfo, error) {
	multiAddress := pinfo.PandoMultiAddr + "/ipfs/" + pinfo.PandoPeerID
	peerInfo, err := peer.AddrInfoFromString(multiAddress)
	if err != nil {
		return nil, err
	}
	return peerInfo, nil
}

func GetPandoInfo() (*PandoInfo, error) {
	res, err := http.Get(kenlbasInfoUrl)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wrong http code: %v", res.StatusCode)
	}
	pinfo := new(httpRes)
	err = json.NewDecoder(res.Body).Decode(pinfo)
	if err != nil {
		return nil, err
	}
	return &PandoInfo{
		PandoMultiAddr: pinfo.Data.APIAddresses.GRAPHSYNC_API,
		PandoPeerID:    pinfo.Data.PeerID,
		PandoAPIUrl:    pandoAPIUrl,
		TopicName:      "/pando/v0.0.1",
	}, nil
}

func NewPandoInfo() PandoInfo {
	pinfo, err := GetPandoInfo()
	if err != nil {
		fmt.Printf("failed to get PandoInfo from Kenlabs http api...Please fill manually, err: %v", err.Error())
		return PandoInfo{TopicName: "/pando/v0.0.1"}
	}
	return *pinfo
}
