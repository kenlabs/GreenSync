package monitor

import (
	"GreenSync/pkg/config"
	"GreenSync/pkg/types/schema/location"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"net/http"
	"time"
)

const (
	EpochKey = "/EPOCH"
)

var log = logging.Logger("monitor")

type Monitor struct {
	Epoch     uint64
	DS        datastore.Batching
	lsys      *ipld.LinkSystem
	greenInfo *config.GreenInfo
	taskCh    chan cid.Cid
	ctx       context.Context
	cncl      context.CancelFunc
}

func New(ctx context.Context, greenInfo *config.GreenInfo, lsys *ipld.LinkSystem, taskCh chan cid.Cid, ds datastore.Batching) (*Monitor, error) {
	cctx, cncl := context.WithCancel(ctx)
	m := &Monitor{
		greenInfo: greenInfo,
		DS:        ds,
		lsys:      lsys,
		taskCh:    taskCh,
		ctx:       cctx,
		cncl:      cncl,
	}

	err := m.init()
	if err != nil {
		log.Errorf("failed to initlize monitor, err:%v", err)
		return nil, err
	}

	go m.monitor()

	return m, nil
}

func (m *Monitor) init() error {
	epochBytes, err := m.DS.Get(context.Background(), datastore.NewKey(EpochKey))
	if err == nil {
		m.Epoch = binary.BigEndian.Uint64(epochBytes)
		log.Infof("load the epoch: %v", m.Epoch)
	} else if err != datastore.ErrNotFound {
		return err
	}
	return nil
}

func (m *Monitor) monitor() {
	interval, err := time.ParseDuration(m.greenInfo.CheckInterval)
	if err != nil {
		log.Errorf("valid check interval, err: %v", err)
		m.Close()
		return
	}

	for range time.NewTicker(interval).C {
		select {
		case _ = <-m.ctx.Done():
			log.Infof("close gracefully..")
			return
		default:
		}

		res, err := http.Get(m.greenInfo.Url)
		if err != nil {
			log.Errorf("failed to get json from http, err: %v", err)
			continue
		}
		if res.StatusCode != http.StatusOK {
			log.Errorf("wrong http status code: %d", res.StatusCode)
			continue
		}
		var locationRes location.Location
		err = json.NewDecoder(res.Body).Decode(&locationRes)
		if err != nil {
			log.Errorf("failed to read json from http body, err: %v", err)
			continue
		}
		_ = res.Body.Close()
		if locationRes.Epoch == m.Epoch && m.Epoch != 0 {
			continue
		}
		err = m.generateAndUpdate(m.ctx, &locationRes)
		if err != nil {
			log.Errorf("some errors happened while generating IPLD node and update to Pando, err: %v", err)
			continue
		}
		_ = m.updateEpoch(m.ctx, locationRes.Epoch)
	}
}

func (m *Monitor) generateAndUpdate(ctx context.Context, l *location.Location) error {
	lnode, err := l.ToNode()
	if err != nil {
		log.Errorf("failed to marshal Location to ipld node, err: %v", err)
		return err
	}
	link, err := m.lsys.Store(ipld.LinkContext{}, location.LinkProto, lnode)
	if err != nil {
		log.Errorf("failed to save Location, err: %s", err)
		return err
	}
	select {
	case _ = <-ctx.Done():
		log.Info("context is closed, quit....")
		return nil
	case m.taskCh <- link.(cidlink.Link).Cid:
		log.Infof("push cid: %s to legs", link.(cidlink.Link).Cid.String())
	default:
		log.Errorf("failed to send Location cid to legs")
		return fmt.Errorf("failed to send Location cid to legs")
	}
	return nil
}

func (m *Monitor) updateEpoch(ctx context.Context, epoch uint64) error {
	if m.Epoch == epoch {
		return fmt.Errorf("don't need update")
	}
	m.Epoch = epoch
	ebytes := make([]byte, 8)
	binary.BigEndian.PutUint64(ebytes, epoch)
	err := m.DS.Put(ctx, datastore.NewKey(EpochKey), ebytes)
	if err != nil {
		log.Errorf("failed to update epoch in datastore: %v", err)
	}
	return nil
}

func (m *Monitor) Close() error {
	close(m.taskCh)
	m.cncl()
	return nil
}
