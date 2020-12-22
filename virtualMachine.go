package AVM_TEST

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/consensus/snowman"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/codec"
	"github.com/ava-labs/avalanchego/vms/components/core"
	"time"
)

// This Virtual Machine defines a blockchain that acts as a timestamp server
// Each block contains a piece of data (payload) and the timestamp when it was created
type VM struct {
	core.SnowmanVM

	// codec serializes and de-serializes structs to/from bytes
	codec codec.Codec

	// Proposed pieces of data that haven't been put into a block and proposed yet
	mempool [][32]byte
}

// Initialize this vm
// [ctx] is the execution context
// [db] is this database we read/write
// [toEngine] is used to notify the consensus engine that new blocks are
//   ready to be added to consensus
// The data in the genesis block is [genesisData]
func (vm *VM) Initialize(
	ctx *snow.Context,
	db database.Database,
	genesisData []byte,
	toEngine chan<- common.Message,
	_ []*common.Fx,
) error {
	// First, we initialize the core.SnowmanVM.
	// vm.ParseBlock, which we'll see further on, tells the core.SnowmanVM how to deserialize
	// a block from bytes
	if err := vm.SnowmanVM.Initialize(ctx, db, vm.ParseBlock, toEngine); err != nil {
		ctx.Log.Error("error initializing SnowmanVM: %v", err)
		return err
	}
	// Set vm's codec to a new codec, which we can use to
	// serialize and deserialize blocks
	vm.codec = codec.NewDefault()

	// If the database is empty, initialize the state of this blockchain
	// using the genesis data
	if !vm.DBInitialized() {
		// Ensure that the genesis bytes are no longer than 32 bytes
		// (the genesis block, like all blocks, holds 32 bytes of data)
		if len(genesisData) > 32 {
			return errors.New("genesis data should be bytes (max length 32)")
		}

		// genesisData is a byte slice (because that's what the snowman.VM interface says)
		// but each block contains an byte array.
		// To make the types match, take the first [dataLen] bytes from genesisData
		// and put them in an array
		var genesisDataArr [dataLen]byte
		copy(genesisDataArr[:], genesisData)

		// Create the genesis block
		// Timestamp of genesis block is 0. It has no parent, so we say the parent's ID is empty.
		// We'll come to the definition of NewBlock later.
		genesisBlock, err := vm.NewBlock(ids.Empty, genesisDataArr, time.Unix(0, 0))
		if err != nil {
			vm.Ctx.Log.Error("error while creating genesis block: %v", err)
			return err
		}

		// Persist the genesis block to the database.
		// Normally, a block is saved to the database when Verify() is called on the block.
		// We don't call Verify on the genesis block, though. (It has no parent so
		// it wouldn't pass verification.)
		// vm.DB is the database, and was set when we initialized the embedded SnowmanVM.
		if err := vm.SaveBlock(vm.DB, genesisBlock); err != nil {
			vm.Ctx.Log.Error("error while saving genesis block: %v", err)
			return err
		}

		// Accept the genesis block.
		// Sets [vm.lastAccepted] and [vm.preferred] to the genesisBlock.
		genesisBlock.Accept()

		// Mark the database as initialized so that in the future when this chain starts
		// it pulls state from the database rather than starting over from genesis
		vm.SetDBInitialized()

		// Flush the database
		if err := vm.DB.Commit(); err != nil {
			vm.Ctx.Log.Error("error while commiting db: %v", err)
			return err
		}
	}
	return nil
}

// proposeBlock appends [data] to [p.mempool].
// Then it notifies the consensus engine
// that a new block is ready to be added to consensus
// (namely, a block with data [data])
func (vm *VM) proposeBlock(data [dataLen]byte) {
	vm.mempool = append(vm.mempool, data)
	vm.NotifyBlockReady()
}

// ParseBlock parses [bytes] to a snowman.Block
// This function is used by the vm's state to unmarshal blocks saved in state
// and by the consensus layer when it receives the byte representation of a block
// from another node
func (vm *VM) ParseBlock(bytes []byte) (snowman.Block, error) {
	// A new empty block
	block := &Block{}

	// Unmarshal the byte repr. of the block into our empty block
	err := vm.codec.Unmarshal(bytes, block)

	// Initialize the block
	// (Block inherits Initialize from its embedded *core.Block)
	block.Initialize(bytes, &vm.SnowmanVM)
	return block, err
}

// NewBlock returns a new Block where:
// - the block's parent has ID [parentID]
// - the block's data is [data]
// - the block's timestamp is [timestamp]
func (vm *VM) NewBlock(parentID ids.ID, data [dataLen]byte, timestamp time.Time) (*Block, error) {
	// Create our new block
	block := &Block{
		Block:     core.NewBlock(parentID),
		Data:      data,
		Timestamp: timestamp.Unix(),
	}

	// Get the byte representation of the block
	blockBytes, err := vm.codec.Marshal(block)
	if err != nil {
		return nil, err
	}

	// Initialize the block by providing it with its byte representation
	// and a reference to SnowmanVM
	block.Initialize(blockBytes, &vm.SnowmanVM)

	return block, nil
}

// BuildBlock returns a block that this VM wants to add to consensus
func (vm *VM) BuildBlock() (snowman.Block, error) {
	// There is no data to put in a new block
	if len(vm.mempool) == 0 {
		return nil, errors.New("there is no block to propose")
	}

	// Get the value to put in the new block
	value := vm.mempool[0]
	vm.mempool = vm.mempool[1:]

	// Notify consensus engine that there are more pending data for blocks
	// (if that is the case) when done building this block
	if len(vm.mempool) > 0 {
		defer vm.NotifyBlockReady()
	}

	// Build the block
	block, err := vm.NewBlock(vm.Preferred(), value, time.Now())
	if err != nil {
		return nil, err
	}
	return block, nil
}

// CreateHandlers returns a map where:
// Keys: The path extension for this blockchain's API (empty in this case)
// Values: The handler for the API
// In this case, our blockchain has only one API, which we name timestamp,
// and it has no path extension, so the API endpoint:
// [Node IP]/ext/bc/[this blockchain's ID]
// See API section in documentation for more information
func (vm *VM) CreateHandlers() map[string]*common.HTTPHandler {
	// Create the API handler (we'll see the declaration of Service further on)
	handler, _ := vm.NewHandler("timestamp", &Service{vm})
	return map[string]*common.HTTPHandler{
		"": handler,
	}
}
