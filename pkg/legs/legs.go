package legs

import (
	"GreenSync/pkg/config"
	"GreenSync/pkg/linksystem"
	"GreenSync/pkg/types/schema/location"
	"GreenSync/pkg/util"
	"context"
	"encoding/json"
	"fmt"
	"github.com/filecoin-project/go-legs"
	"github.com/filecoin-project/go-legs/dtsync"
	"github.com/go-resty/resty/v2"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"net/http"
	"sync"
	"time"
)

var log = logging.Logger("ProviderLegs")

var (
	latestMetaKey = datastore.NewKey("/latestMetaKey")
	checkListKey  = datastore.NewKey("/checkListKey")
)

type ProviderLegs struct {
	Publisher         legs.Publisher
	DS                *dssync.MutexDatastore
	host              host.Host
	PandoInfo         *config.PandoInfo
	httpClient        *resty.Client
	syncCfg           *config.IngestCfg
	latestMeta        cid.Cid
	lsys              *ipld.LinkSystem
	taskQueue         chan cid.Cid
	checkMutex        sync.Mutex
	checkLocationList map[string]struct{}
	ctx               context.Context
	cncl              context.CancelFunc
}

func New(ctx context.Context, pinfo *config.PandoInfo, syncCfg *config.IngestCfg, h host.Host, ds *dssync.MutexDatastore, lsys *ipld.LinkSystem) (*ProviderLegs, error) {
	cctx, cncl := context.WithCancel(ctx)
	p := &ProviderLegs{
		DS:                ds,
		host:              h,
		PandoInfo:         pinfo,
		syncCfg:           syncCfg,
		lsys:              lsys,
		taskQueue:         make(chan cid.Cid, 0),
		checkLocationList: make(map[string]struct{}),
		ctx:               cctx,
		cncl:              cncl,
	}
	err := p.init()
	if err != nil {
		return nil, err
	}

	go p.start()
	go p.checkSyncStatus()

	return p, nil
}

func initWithPando(pinfo *config.PandoInfo, h host.Host) error {
	peerID, err := peer.Decode(pinfo.PandoPeerID)
	if err != nil {
		return err
	}

	connections := h.Network().Connectedness(peerID)
	if connections != network.Connected {
		peerInfo, err := pinfo.AddrInfo()
		if err != nil {
			return err
		}
		if err = h.Connect(context.Background(), *peerInfo); err != nil {
			log.Errorf("failed to connect with Pando libp2p host, err:%v", err)
			return err
		}
	}
	h.ConnManager().Protect(peerID, "PANDO")
	return nil
}

func (p *ProviderLegs) init() error {
	// create legs provider
	legsPublisher, err := dtsync.NewPublisher(p.host, p.DS, *p.lsys, p.PandoInfo.Topic)
	// load latest local location cid
	var latestMetaCid cid.Cid
	metaCid, err := p.DS.Get(context.Background(), latestMetaKey)
	if err == nil {
		_, latestMetaCid, err = cid.CidFromBytes(metaCid)
		if err != nil {
			return err
		}
	} else if err != nil && err != datastore.ErrNotFound {
		return err
	}
	// load check list to []cid.Cid
	var checkList map[string]struct{}
	checkListBytes, err := p.DS.Get(context.Background(), checkListKey)
	if err == nil {
		_err := json.Unmarshal(checkListBytes, &checkList)
		if _err != nil {
			return _err
		}
		p.checkLocationList = checkList
	} else if err != nil && err != datastore.ErrNotFound {
		return err
	}
	p.Publisher = legsPublisher
	p.latestMeta = latestMetaCid
	p.httpClient = resty.New().SetBaseURL(p.PandoInfo.PandoAPIUrl).SetTimeout(time.Second * 10).SetDebug(false)

	err = initWithPando(p.PandoInfo, p.host)
	if err != nil {
		return err
	}

	return nil
}

func (p *ProviderLegs) start() {
	for {
		select {
		case _ = <-p.ctx.Done():
			log.Info("close gracefully.")
			return
		case c, ok := <-p.taskQueue:
			if !ok {
				log.Warn("task queue is closed, quit....")
				return
			}
			metaCid, err := p.UpdateLocationToPando(c)
			if err != nil {
				log.Errorf("failed to update location to Pando, err: %v", err)
			}
			err = p.updateCheckList([]string{metaCid.String()}, nil)
			if err != nil {
				log.Errorf("failed to add key: %s to check list, err: %v", c.String(), err)
			}
		}
	}
}

func (p *ProviderLegs) checkSyncStatus() {
	interval, err := time.ParseDuration(p.syncCfg.SyncCheckInterval)
	if err != nil {
		_err := p.Close()
		if _err != nil {
			log.Errorf("failed to quit gracefully, err:%v", _err)
		}
	}

	for range time.NewTicker(interval).C {
		for k, _ := range p.checkLocationList {
			res, err := handleResError(p.httpClient.R().Get("/metadata/inclusion?cid=" + k))
			if err != nil {
				log.Errorf("failed to get meta inclusion from Pando, err: %v", err)
				continue
			}
			resJson := responseJson{}
			err = json.Unmarshal(res.Body(), &resJson)
			if err != nil {
				log.Errorf("failed to unmarshal meta inclusion, err: %v", err)
				continue
			}
			if resJson.Code != http.StatusOK {
				log.Errorf("error msg: %s", resJson.Message)
				continue
			}
			if resJson.Data.InPando && resJson.Data.InSnapShot {
				log.Debugf("Location: %s sync successfully!", k)
				err = p.updateCheckList(nil, []string{k})
				if err != nil {
					log.Errorf("failed to delete cid: %s from check list", k)
				}
			}
		}
	}

}

func (p *ProviderLegs) Close() error {
	p.cncl()
	return p.Publisher.Close()
}

func (p *ProviderLegs) UpdateLocationToPando(c cid.Cid) (cid.Cid, error) {
	n, err := p.lsys.Load(ipld.LinkContext{}, cidlink.Link{Cid: c}, location.LocationPrototype)
	if err != nil {
		log.Errorf("failed to load Location node from linksystem, err: %v", err)
		return cid.Undef, err
	}
	if !linksystem.IsLocation(n) {
		log.Warnf("received unexpected ipld node(expected Location), skip workflow")
		return cid.Undef, nil
	}
	l, err := location.UnwrapLocation(n)
	if err != nil {
		log.Errorf("failed to unmarshal location from ipld node, err : %v", err)
		return cid.Undef, nil
	}
	link := ipld.Link(cidlink.Link{Cid: p.latestMeta})
	var previousID *ipld.Link
	if p.latestMeta.Equals(cid.Undef) {
		previousID = nil
	} else {
		previousID = &link
	}
	meta := &location.LocationMeta{
		PreviousID: previousID,
		Provider:   p.host.ID().String(),
		Payload:    *l,
		Signature:  nil,
	}
	sig, err := util.SignWithPrivky(p.host.Peerstore().PrivKey(p.host.ID()), meta)
	if err != nil {
		log.Errorf("failed to sign the locationMeta, err: %v", err)
		return cid.Undef, err
	}
	meta.Signature = sig
	mnode, err := meta.ToNode()
	if err != nil {
		log.Errorf("failed to save locationMeta to ipld node, err: %v", err)
		return cid.Undef, err
	}
	lnk, err := p.lsys.Store(ipld.LinkContext{}, location.LinkProto, mnode)
	if err != nil {
		log.Errorf("failed to save LocationMeta to linksystem, err: %v", err)
		return cid.Undef, err
	}

	err = p.Publisher.UpdateRoot(context.Background(), lnk.(cidlink.Link).Cid)
	if err != nil {
		log.Errorf("failed to update root by legs, err: %v", err)
		return cid.Undef, err
	}
	err = p.updateLatestMeta(lnk.(cidlink.Link).Cid)
	if err != nil {
		log.Errorf("failed to update latest meta cid, err: %v", err)
		return cid.Undef, err
	}

	return lnk.(cidlink.Link).Cid, nil
}

func (p *ProviderLegs) updateLatestMeta(c cid.Cid) error {
	if c == cid.Undef {
		return fmt.Errorf("meta cid can not be nil")
	}
	p.latestMeta = c
	return p.DS.Put(context.Background(), latestMetaKey, c.Bytes())
}

func (p *ProviderLegs) GetTaskQueue() chan cid.Cid {
	return p.taskQueue
}

func (p *ProviderLegs) updateCheckList(addList []string, delList []string) error {
	p.checkMutex.Lock()
	defer p.checkMutex.Unlock()
	if addList != nil || len(addList) != 0 {
		for _, c := range addList {
			_, ok := p.checkLocationList[c]
			if ok {
				// todo: repeat Location?
				continue
			}
			p.checkLocationList[c] = struct{}{}
		}
	}
	if delList != nil || len(delList) != 0 {
		for _, c := range delList {
			_, ok := p.checkLocationList[c]
			if !ok {
				return fmt.Errorf("key: %s not exist to delete", c)
			}
			delete(p.checkLocationList, c)
		}
	}
	listBytes, err := json.Marshal(p.checkLocationList)
	if err != nil {
		log.Errorf("failed to marshal checklist, err:%v", err)
		return err
	}
	err = p.DS.Put(p.ctx, checkListKey, listBytes)
	if err != nil {
		return err
	}

	return nil
}

func handleResError(res *resty.Response, err error) (*resty.Response, error) {
	errTmpl := "failed to get latest head, error: %v"
	if err != nil {
		return res, err
	}
	if res.IsError() {
		return res, fmt.Errorf(errTmpl, res.Error())
	}
	if res.StatusCode() != http.StatusOK {
		return res, fmt.Errorf(errTmpl, fmt.Sprintf("expect 200, got %d", res.StatusCode()))
	}

	return res, nil
}

type MetaInclusion struct {
	ID             cid.Cid `json:"ID"`
	Provider       peer.ID `json:"Provider"`
	InPando        bool    `json:"InPando"`
	InSnapShot     bool    `json:"InSnapShot"`
	SnapShotID     cid.Cid `json:"SnapShotID"`
	SnapShotHeight uint64  `json:"SnapShotHeight"`
	Context        []byte  `json:"Context"`
	TranscationID  int     `json:"TranscationID"`
}

type responseJson struct {
	Code    int           `json:"code"`
	Message string        `json:"message"`
	Data    MetaInclusion `json:"Data"`
}
