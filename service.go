package AVM_TEST

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"net/http"
)

// Service is the API service for this VM
type Service struct{ vm *VM }

// ProposeBlockArgs are the arguments to ProposeValue
type ProposeBlockArgs struct {
	// Data for the new block. Must be base 58 encoding (with checksum) of 32 bytes.
	Data string
}

// ProposeBlockReply is the reply from function ProposeBlock
type ProposeBlockReply struct {
	// True if the operation was successful
	Success bool
}

// ProposeBlock is an API method to propose a new block whose data is [args].Data.
func (s *Service) ProposeBlock(_ *http.Request, args *ProposeBlockArgs, reply *ProposeBlockReply) error {
	// Parse the data given as argument to bytes
	byteFormatter := formatting.CB58{}
	if err := byteFormatter.FromString(args.Data); err != nil {
		return errBadData
	}
	// Ensure the data is 32 bytes
	dataSlice := byteFormatter.Bytes
	if len(dataSlice) != 32 {
		return errBadData
	}
	// Convert the data from a byte slice to byte array
	var data [dataLen]byte
	copy(data[:], dataSlice[:dataLen])
	// Invoke proposeBlock to trigger creation of block with this data
	s.vm.proposeBlock(data)
	reply.Success = true
	return nil
}

// APIBlock is the API representation of a block
type APIBlock struct {
	Timestamp int64  `json:"timestamp"` // Timestamp of most recent block
	Data      string `json:"data"`      // Data in the most recent block. Base 58 repr. of 5 bytes.
	ID        string `json:"id"`        // String repr. of ID of the most recent block
	ParentID  string `json:"parentID"`  // String repr. of ID of the most recent block's parent
}

// GetBlockArgs are the arguments to GetBlock
type GetBlockArgs struct {
	// ID of the block we're getting.
	// If left blank, gets the latest block
	ID string
}

// GetBlockReply is the reply from GetBlock
type GetBlockReply struct {
	APIBlock
}

// GetBlock gets the block whose ID is [args.ID]
// If [args.ID] is empty, get the latest block
func (s *Service) GetBlock(_ *http.Request, args *GetBlockArgs, reply *GetBlockReply) error {
	// If an ID is given, parse its string representation to an ids.ID
	// If no ID is given, ID becomes the ID of last accepted block
	var ID ids.ID
	var err error
	if args.ID == "" {
		ID = s.vm.LastAccepted()
	} else {
		ID, err = ids.FromString(args.ID)
		if err != nil {
			return errors.New("problem parsing ID")
		}
	}

	// Get the block from the database
	blockInterface, err := s.vm.GetBlock(ID)
	if err != nil {
		return errors.New("error getting data from database")
	}

	block, ok := blockInterface.(*Block)
	if !ok { // Should never happen but better to check than to panic
		return errors.New("error getting data from database")
	}

	// Fill out the response with the block's data
	reply.APIBlock.ID = block.ID().String()
	reply.APIBlock.Timestamp = block.Timestamp
	reply.APIBlock.ParentID = block.ParentID().String()
	byteFormatter := formatting.CB58{Bytes: block.Data[:]}
	reply.Data = byteFormatter.String()

	return nil
}
