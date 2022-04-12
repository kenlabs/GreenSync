package location

import (
	_ "embed"
	"fmt"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/ipld/go-ipld-prime/schema"
	"github.com/multiformats/go-multicodec"
)

var (
	// Linkproto is the ipld.LinkProtocol used for the legs protocol.
	// Refer to it if you have encoding questions.
	LinkProto = cidlink.LinkPrototype{
		Prefix: cid.Prefix{
			Version:  1,
			Codec:    uint64(multicodec.DagJson),
			MhType:   uint64(multicodec.Sha2_256),
			MhLength: 16,
		},
	}

	// MetadataPrototype represents the IPLD node prototype of Metadata.
	// See: bindnode.Prototype.
	LocationPrototype schema.TypedPrototype
	//LocationMetaPrototype schema.TypedPrototype
	//MinerLocationPrototype schema.TypedPrototype
	//go:embed schema.ipldsch
	schemaBytes []byte
)

func init() {
	typeSystem, err := ipld.LoadSchemaBytes(schemaBytes)
	if err != nil {
		panic(fmt.Errorf("failed to load schema: %w", err))
	}
	LocationPrototype = bindnode.Prototype((*Location)(nil), typeSystem.TypeByName("Location"))
	//LocationMetaPrototype = bindnode.Prototype((*LocationMeta)(nil), typeSystem.TypeByName("LocationMeta"))
	//MinerLocationPrototype = bindnode.Prototype((*MinerLocation)(nil), typeSystem.TypeByName("MinerLocation"))

}
