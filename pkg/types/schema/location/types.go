package location

import (
	"errors"
	"fmt"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/node/bindnode"
)

type Location struct {
	Date           string           `json:"date"`
	Epoch          uint64           `json:"epoch"`
	MinerLocations []*MinerLocation `json:"minerLocations"`
}

type MinerLocation struct {
	Miner        string  `json:"miner"`
	Region       string  `json:"region"`
	Long         float32 `json:"long"`
	Lat          float32 `json:"lat"`
	NumLocations int     `json:"numLocations"`
	Country      string  `json:"country"`
	City         string  `json:"city"`
	SubDiv1      string  `json:"subdiv1"`
}

type LocationMeta struct {
	PreviousID *ipld.Link
	Provider   string
	Payload    Location
	Signature  []byte
}

func (lm *LocationMeta) ToNode() (n ipld.Node, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = toError(r)
		}
	}()
	n = bindnode.Wrap(lm, LocationMetaPrototype.Type()).Representation()
	return
}

// ToNode converts this metadata to its representation as an IPLD typed node.
// See: bindnode.Wrap.
func (l *Location) ToNode() (n ipld.Node, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = toError(r)
		}
	}()
	n = bindnode.Wrap(l, LocationPrototype.Type()).Representation()
	return
}

// UnwrapMetadata unwraps the given node as metadata.
//
// Note that the node is reassigned to MetadataPrototype if its prototype is different.
// Therefore, it is recommended to load the node using the correct prototype initially
// function to avoid unnecessary node assignment.
func UnwrapLocation(node ipld.Node) (*Location, error) {
	// When an IPLD node is loaded using `Prototype.Any` unwrap with bindnode will not work.
	// Here we defensively check the prototype and wrap if needed, since:
	//   - linksystem in sti is passed into other libraries, like go-legs, and
	//   - for whatever reason clients of this package may load nodes using Prototype.Any.
	//
	// The code in this repo, however should load nodes with appropriate prototype and never trigger
	// this if statement.
	if node.Prototype() != LocationPrototype {
		adBuilder := LocationPrototype.NewBuilder()
		err := adBuilder.AssignNode(node)
		if err != nil {
			return nil, fmt.Errorf("faild to convert node prototype: %w", err)
		}
		node = adBuilder.Build()
	}

	ad, ok := bindnode.Unwrap(node).(*Location)
	if !ok || ad == nil {
		return nil, fmt.Errorf("unwrapped node does not match schema.Metadata")
	}
	return ad, nil
}

func toError(r interface{}) error {
	switch x := r.(type) {
	case string:
		return errors.New(x)
	case error:
		return x
	default:
		return fmt.Errorf("unknown panic: %v", r)
	}
}
