package AVM_TEST

import (
	"github.com/ava-labs/avalanchego/vms/components/core"
	"time"
)

// Block is a block on the chain.
// Each block contains:
// 1) A piece of data (the block's payload)
// 2) The (unix) timestamp when the block was created
type Block struct {
	*core.Block `serialize:"true"`
	Data        [32]byte `serialize:"true"`
	Timestamp   int64    `serialize:"true"`
}

// Verify returns nil iff this block is valid.
// To be valid, it must be that:
// b.parent.Timestamp < b.Timestamp <= [local time] + 1 hour
func (b *Block) Verify() error {
	// Check to see if this block has already been verified by calling Verify on the
	// embedded *core.Block.
	// If there is an error while checking, return an error.
	// If the core.Block says the block is accepted, return accepted.
	if accepted, err := b.Block.Verify(); err != nil || accepted {
		return err
	}

	// Get [b]'s parent
	parent, ok := b.Parent().(*Block)
	if !ok {
		return errors.New("error while retrieving block from database")
	}

	// Ensure [b]'s timestamp is after its parent's timestamp.
	if b.Timestamp < time.Unix(parent.Timestamp, 0).Unix() {
		return errors.New("block's timestamp is more than 1 hour ahead of local time")
	}

	// Ensure [b]'s timestamp is not more than an hour
	// ahead of this node's time
	if b.Timestamp >= time.Now().Add(time.Hour).Unix() {
		return errors.New("block's timestamp is more than 1 hour ahead of local time")
	}

	// Our block inherits VM from *core.Block.
	// It holds the database we read/write, b.VM.DB
	// We persist this block to that database using VM's SaveBlock method.
	b.VM.SaveBlock(b.VM.DB, b)

	// Then we flush the database's contents
	return b.VM.DB.Commit()
}
