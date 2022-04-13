package linksystem

import (
	"bytes"
	"errors"
	"fmt"
	blocks "github.com/ipfs/go-block-format"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/multicodec"
	"github.com/ipld/go-ipld-prime/node/basicnode"
	"io"
)

var log = logging.Logger("link-system")

func MkLinkSystem(bs blockstore.Blockstore) *ipld.LinkSystem {
	lsys := cidlink.DefaultLinkSystem()
	lsys.TrustedStorage = true
	lsys.StorageReadOpener = func(lnkCtx ipld.LinkContext, lnk ipld.Link) (io.Reader, error) {
		asCidLink, ok := lnk.(cidlink.Link)
		if !ok {
			return nil, fmt.Errorf("unsupported link types")
		}
		block, err := bs.Get(lnkCtx.Ctx, asCidLink.Cid)
		if err != nil {
			return nil, err
		}
		return bytes.NewBuffer(block.RawData()), nil
	}
	lsys.StorageWriteOpener = func(lctx ipld.LinkContext) (io.Writer, ipld.BlockWriteCommitter, error) {
		buf := bytes.NewBuffer(nil)
		return buf, func(lnk ipld.Link) error {
			c := lnk.(cidlink.Link).Cid
			codec := lnk.(cidlink.Link).Prefix().Codec
			origBuf := buf.Bytes()

			log := log.With("cid", c)

			// Decode the node to check its type.
			n, err := decodeIPLDNode(codec, buf, basicnode.Prototype.Any)
			if err != nil {
				log.Errorw("Error decoding IPLD node in linksystem", "err", err)
				return errors.New("bad ipld data")
			}
			if IsLocation(n) {
				log.Infow("Received Location")
			} else if IsLocationMeta(n) {
				log.Infow("Received LocationMeta")
			} else {
				log.Debug("Received unexpected IPLD node, skip")
				return nil
			}
			block, err := blocks.NewBlockWithCid(origBuf, c)
			if err != nil {
				return err
			}
			return bs.Put(lctx.Ctx, block)

		}, nil
	}
	return &lsys
}

// decodeIPLDNode decodes an ipld.Node from bytes read from an io.Reader.
func decodeIPLDNode(codec uint64, r io.Reader, prototype ipld.NodePrototype) (ipld.Node, error) {
	// NOTE: Considering using the schema prototypes.  This was failing, using
	// a map gives flexibility.  Maybe is worth revisiting this again in the
	// future.
	nb := prototype.NewBuilder()
	decoder, err := multicodec.LookupDecoder(codec)
	if err != nil {
		return nil, err
	}
	err = decoder(nb, r)
	if err != nil {
		return nil, err
	}
	return nb.Build(), nil
}

func IsLocation(n ipld.Node) bool {
	Date, _ := n.LookupByString("Date")
	Epoch, _ := n.LookupByString("Epoch")
	MinerLocations, _ := n.LookupByString("MinerLocations")
	return Date != nil && Epoch != nil && MinerLocations != nil
}

func IsLocationMeta(n ipld.Node) bool {
	provider, _ := n.LookupByString("Provider")
	location, _ := n.LookupByString("Payload")
	signature, _ := n.LookupByString("Signature")
	return provider != nil && location != nil && signature != nil
}
