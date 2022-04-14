package monitor

import (
	"PandoWatch/pkg/types/schema/location"
	"context"
	"encoding/json"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"net/http"
	"strconv"
	"time"
)

const (
	EpochKey = "/EPOCH"
)

var log = logging.Logger("monitor")

type Monitor struct {
	Url    string
	Epoch  uint64
	DS     datastore.Batching
	lsys   *ipld.LinkSystem
	taskCh chan cid.Cid
	ctx    context.Context
	cncl   context.CancelFunc
}

func New(ctx context.Context, url string, lsys *ipld.LinkSystem, taskCh chan cid.Cid, ds datastore.Batching) (*Monitor, error) {
	var epoch uint64
	epochBytes, err := ds.Get(context.Background(), datastore.NewKey(EpochKey))
	if err == nil {
		epoch, err = strconv.ParseUint(string(epochBytes), 10, 64)
		if err != nil {
			return nil, err
		}
	} else if err != datastore.ErrNotFound {
		return nil, err
	}

	cctx, cncl := context.WithCancel(ctx)

	m := &Monitor{
		Url:    url,
		Epoch:  epoch,
		DS:     ds,
		lsys:   lsys,
		taskCh: taskCh,
		ctx:    cctx,
		cncl:   cncl,
	}

	go m.monitor()

	return m, nil
}

func (m *Monitor) monitor() {
	interval := time.Second
	for range time.NewTicker(interval).C {
		select {
		case _ = <-m.ctx.Done():
			log.Infof("close gracefully..")
			return
		default:
		}

		res, err := http.Get(m.Url)
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
		if locationRes.Epoch == m.Epoch && m.Epoch != 0 {
			continue
		}
		m.GenerateAndUpdate(m.ctx, &locationRes)
	}
}

func (m *Monitor) GenerateAndUpdate(ctx context.Context, l *location.Location) {
	lnode, err := l.ToNode()
	if err != nil {
		log.Errorf("failed to marshal Location to ipld node, err: %v", err)
	}
	link, err := m.lsys.Store(ipld.LinkContext{}, location.LinkProto, lnode)
	if err != nil {
		log.Errorf("failed to save Location, err: %s", err)
		return
	}
	select {
	case m.taskCh <- link.(cidlink.Link).Cid:
		log.Infof("push cid: %s to legs", link.(cidlink.Link).Cid.String())
	default:
		log.Errorf("failed to send Location cid to legs")
	}

}

func (m *Monitor) Close() error {
	close(m.taskCh)
	m.cncl()
	return nil
}
