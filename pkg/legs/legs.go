package legs

import (
	"PandoWatch/pkg/linksystem"
	"PandoWatch/pkg/types/schema/location"
	"PandoWatch/pkg/util"
	"context"
	"fmt"
	"github.com/filecoin-project/go-legs"
	"github.com/filecoin-project/go-legs/dtsync"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

var log = logging.Logger("ProviderLegs")

var LatestMetaKey = datastore.NewKey("/latestMetaKey")

type PandoInfo struct {
	PandoMultiAddr ma.Multiaddr
	PandoPeerID    peer.ID
	Topic          string
}

type ProviderLegs struct {
	Publisher  legs.Publisher
	Ds         datastore.Batching
	host       host.Host
	PandoInfo  *PandoInfo
	latestMeta cid.Cid
	lsys       *ipld.LinkSystem
	taskQueue  chan cid.Cid
	ctx        context.Context
	cncl       context.CancelFunc
}

func New(ctx context.Context, pinfo *PandoInfo, h host.Host, ds datastore.Batching, lsys *ipld.LinkSystem) (*ProviderLegs, error) {
	dstore := dssync.MutexWrap(ds)
	legsPublisher, err := dtsync.NewPublisher(h, dstore, *lsys, pinfo.Topic)
	var latestMetaCid cid.Cid
	metaCid, err := dstore.Get(context.Background(), LatestMetaKey)
	if err == nil {
		_, latestMetaCid, err = cid.CidFromBytes(metaCid)
		if err != nil {
			return nil, err
		}
	} else if err != nil && err != datastore.ErrNotFound {
		return nil, err
	}
	err = initWithPando(pinfo, h)
	if err != nil {
		return nil, err
	}

	cctx, cncl := context.WithCancel(ctx)

	p := &ProviderLegs{
		Publisher:  legsPublisher,
		Ds:         dstore,
		host:       h,
		PandoInfo:  pinfo,
		lsys:       lsys,
		latestMeta: latestMetaCid,
		taskQueue:  make(chan cid.Cid, 0),
		ctx:        cctx,
		cncl:       cncl,
	}
	go p.Start()

	return p, nil
}

func initWithPando(pinfo *PandoInfo, h host.Host) error {
	multiAddress := pinfo.PandoMultiAddr.String() + "/ipfs/" + pinfo.PandoPeerID.String()
	peerInfo, err := peer.AddrInfoFromString(multiAddress)
	if err != nil {
		return err
	}
	if err = h.Connect(context.Background(), *peerInfo); err != nil {
		return err
	}
	return nil
}

func (p *ProviderLegs) Start() {
	for {
		select {
		case _ = <-p.ctx.Done():
			log.Info("close gracefully.")
			break
		case c, ok := <-p.taskQueue:
			if !ok {
				log.Errorf("task queue is closed, quit....")
				break
			}
			err := p.UpdateLocationToPando(c)
			if err != nil {
				log.Errorf("failed to update location to Pando, err: %v", err)
				continue
			}
		}
	}
}

func (p *ProviderLegs) Close() error {
	close(p.taskQueue)
	p.cncl()
	return p.Publisher.Close()
}

func (p *ProviderLegs) UpdateLocationToPando(c cid.Cid) error {
	n, err := p.lsys.Load(ipld.LinkContext{}, cidlink.Link{Cid: c}, location.LocationPrototype)
	if err != nil {
		log.Errorf("failed to load Location node from linksystem, err: %v", err)
		return err
	}
	if !linksystem.IsLocation(n) {
		log.Warnf("received unexpected ipld node(expected Location), skip workflow")
		return nil
	}
	l, err := location.UnwrapLocation(n)
	if err != nil {
		log.Errorf("failed to unmarshal location from ipld node, err : %v", err)
		return nil
	}
	link := ipld.Link(cidlink.Link{Cid: p.latestMeta})
	meta := &location.LocationMeta{
		PreviousID: &link,
		Provider:   p.host.ID().String(),
		Payload:    *l,
		Signature:  nil,
	}
	sig, err := util.SignWithPrivky(p.host.Peerstore().PrivKey(p.host.ID()), meta)
	if err != nil {
		log.Errorf("failed to sign the locationMeta, err: %v", err)
		return err
	}
	meta.Signature = sig
	mnode, err := meta.ToNode()
	if err != nil {
		log.Errorf("failed to save locationMeta to ipld node, err: %v", err)
		return err
	}
	lnk, err := p.lsys.Store(ipld.LinkContext{}, location.LinkProto, mnode)
	if err != nil {
		log.Errorf("failed to save LocationMeta to linksystem, err: %v", err)
		return err
	}

	err = p.Publisher.UpdateRoot(context.Background(), lnk.(cidlink.Link).Cid)
	if err != nil {
		log.Errorf("failed to update root by legs, err: %v", err)
		return err
	}
	err = p.updateLatestMeta(lnk.(cidlink.Link).Cid)
	if err != nil {
		log.Errorf("failed to update latest meta cid, err: %v", err)
		return err
	}

	return nil
}

func (p *ProviderLegs) updateLatestMeta(c cid.Cid) error {
	if c == cid.Undef {
		return fmt.Errorf("meta cid can not be nil")
	}
	p.latestMeta = c
	return p.Ds.Put(context.Background(), LatestMetaKey, c.Bytes())
}

func (p *ProviderLegs) GetTaskQueue() chan cid.Cid {
	return p.taskQueue
}
