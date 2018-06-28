package siafile

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"sync"

	"github.com/NebulousLabs/Sia/build"
	"github.com/NebulousLabs/Sia/modules"
	"github.com/NebulousLabs/Sia/types"
	"github.com/NebulousLabs/fastrand"

	"github.com/NebulousLabs/Sia/crypto"
)

type (
	// SiaFile is the disk format for files uploaded to the Sia network.  It
	// contains all the necessary information to recover a file from its hosts and
	// allows for easy constant-time updates of the file without having to read or
	// write the whole file.
	SiaFile struct {
		// metadata is the mostly static metadata of a SiaFile. The reserved
		// size of the metadata on disk should always be a multiple of 4kib.
		// The metadata is also the only part of the file that is JSON encoded
		// and can therefore be easily extended.
		metadata Metadata

		// pubKeyTable stores the public keys of the hosts this file's pieces are uploaded to.
		// Since multiple pieces from different chunks might be uploaded to the same host, this
		// allows us to deduplicate the rather large public keys.
		pubKeyTable []types.SiaPublicKey

		// chunks are the chunks the file was split into.
		chunks []Chunk

		// utility fields. These are not persisted.
		deleted bool
		mu      sync.RWMutex
		uid     string
	}

	// Chunk represents a single chunk of a file on disk
	Chunk struct {
		// erasure code settings.
		//
		// erasureCodeType specifies the algorithm used for erasure coding
		// chunks. Available types are:
		//   0 - Invalid / Missing Code
		//   1 - Reed Solomon Code
		//
		// erasureCodeParams specifies possible parameters for a certain
		// erasureCodeType. Currently params will be parsed as follows:
		//   Reed Solomon Code - 4 bytes dataPieces / 4 bytes parityPieces
		//
		erasureCodeType   [4]byte
		erasureCodeParams [8]byte
		erasureCode       modules.ErasureCoder

		// extensionInfo is some reserved space for each chunk that allows us
		// to indicate if a chunk is special.
		extensionInfo [16]byte

		// pieces are the pieces of the file the chunk consists of.
		pieces [][]Piece
	}

	// Piece represents a single piece of a chunk on disk
	Piece struct {
		KeyNonce   [4]byte            // nonce used for encrypting the piece
		HostPubKey types.SiaPublicKey // public key of the host
		MerkleRoot crypto.Hash        // merkle root of the piece
	}
)

// New create a new SiaFile.
// TODO needs changes once we move persistence over.
func New(siaPath string, erasureCode []modules.ErasureCoder, pieceSize, fileSize uint64, fileMode os.FileMode, source string) *SiaFile {
	file := &SiaFile{
		metadata: Metadata{
			fileSize:  int64(fileSize),
			localPath: source,
			masterKey: crypto.GenerateTwofishKey(),
			mode:      fileMode,
			pieceSize: pieceSize,
			siaPath:   siaPath,
		},
		uid: string(fastrand.Bytes(20)),
	}
	file.chunks = make([]Chunk, len(erasureCode))
	for i := range file.chunks {
		file.chunks[i].erasureCode = erasureCode[i]
		file.chunks[i].erasureCodeType = [4]byte{0, 0, 0, 1}
		binary.LittleEndian.PutUint32(file.chunks[i].erasureCodeParams[0:4], uint32(erasureCode[i].MinPieces()))
		binary.LittleEndian.PutUint32(file.chunks[i].erasureCodeParams[4:8], uint32(erasureCode[i].NumPieces()-erasureCode[i].MinPieces()))
		file.chunks[i].pieces = make([][]Piece, erasureCode[i].NumPieces())
	}
	return file
}

// AddPiece adds an uploaded piece to the file. It also updates the host table
// if the public key of the host is not aleady known.
// TODO needs changes once we move persistence over.
func (sf *SiaFile) AddPiece(pk types.SiaPublicKey, chunkIndex, pieceIndex uint64, merkleRoot crypto.Hash) error {
	sf.mu.Lock()
	defer sf.mu.Unlock()

	// Get the index of the host in the public key table.
	tableIndex := -1
	for i, hpk := range sf.pubKeyTable {
		if hpk.Algorithm == pk.Algorithm && bytes.Equal(hpk.Key, pk.Key) {
			tableIndex = i
			break
		}
	}
	// If we don't know the host yet, we add it to the table.
	if tableIndex == -1 {
		sf.pubKeyTable = append(sf.pubKeyTable, pk)
		tableIndex = len(sf.pubKeyTable) - 1
	}
	// Check if the chunkIndex is valid.
	if chunkIndex >= uint64(len(sf.chunks)) {
		return fmt.Errorf("chunkIndex %v out of bounds (%v)", chunkIndex, len(sf.chunks))
	}
	// Check if the pieceIndex is valid.
	if pieceIndex >= uint64(len(sf.chunks[chunkIndex].pieces)) {
		return fmt.Errorf("pieceIndex %v out of bounds (%v)", pieceIndex, len(sf.chunks[chunkIndex].pieces))
	}
	// Add the piece to the chunk.
	sf.chunks[chunkIndex].pieces[pieceIndex] = append(sf.chunks[chunkIndex].pieces[pieceIndex], Piece{
		HostPubKey: pk,
		MerkleRoot: merkleRoot,
	})
	return nil
}

// Available indicates whether the file is ready to be downloaded.
func (sf *SiaFile) Available(offline map[string]bool) bool {
	sf.mu.RLock()
	defer sf.mu.RUnlock()
	// We need to find at least erasureCode.MinPieces different pieces for each
	// chunk for the file to be available.
	for chunkIndex, chunk := range sf.chunks {
		piecesForChunk := 0
		for _, pieceSet := range chunk.pieces {
			for _, piece := range pieceSet {
				if !offline[string(piece.HostPubKey.Key)] {
					piecesForChunk++
					break // break out since we only count unique pieces
				}
			}
			if piecesForChunk >= sf.chunks[chunkIndex].erasureCode.MinPieces() {
				break // we already have enough pieces for this chunk.
			}
		}
		if piecesForChunk < sf.chunks[chunkIndex].erasureCode.MinPieces() {
			return false // this chunk isn't available.
		}
	}
	return true
}

// ChunkIndexByOffset will return the chunkIndex that contains the provided
// offset of a file and also the relative offset within the chunk. If the
// offset is out of bounds, chunkIndex will be equal to NumChunk().
func (sf *SiaFile) ChunkIndexByOffset(offset uint64) (chunkIndex uint64, off uint64) {
	for chunkIndex := uint64(0); chunkIndex < uint64(len(sf.chunks)); chunkIndex++ {
		if sf.chunkSize(chunkIndex) > offset {
			return chunkIndex, offset
		}
		offset -= sf.chunkSize(chunkIndex)
	}
	return
}

// ErasureCode returns the erasure coder used by the file.
func (sf *SiaFile) ErasureCode(chunkIndex uint64) modules.ErasureCoder {
	sf.mu.RLock()
	defer sf.mu.RUnlock()
	return sf.chunks[chunkIndex].erasureCode
}

// NumChunks returns the number of chunks the file consists of. This will
// return the number of chunks the file consists of even if the file is not
// fully uploaded yet.
func (sf *SiaFile) NumChunks() uint64 {
	sf.mu.RLock()
	defer sf.mu.RUnlock()
	return uint64(len(sf.chunks))
}

// Pieces returns all the pieces for a chunk in a slice of slices that contains
// all the pieces for a certain index.
func (sf *SiaFile) Pieces(chunkIndex uint64) ([][]Piece, error) {
	sf.mu.RLock()
	defer sf.mu.RUnlock()
	if chunkIndex >= uint64(len(sf.chunks)) {
		panic(fmt.Sprintf("index %v out of bounds (%v)", chunkIndex, len(sf.chunks)))
	}
	// Return a deep-copy to avoid race conditions.
	pieces := make([][]Piece, len(sf.chunks[chunkIndex].pieces))
	for pieceIndex := range pieces {
		pieces[pieceIndex] = make([]Piece, len(sf.chunks[chunkIndex].pieces[pieceIndex]))
		copy(pieces[pieceIndex], sf.chunks[chunkIndex].pieces[pieceIndex])
	}
	return pieces, nil
}

// Redundancy returns the redundancy of the least redundant chunk. A file
// becomes available when this redundancy is >= 1. Assumes that every piece is
// unique within a file contract. -1 is returned if the file has size 0. It
// takes one argument, a map of offline contracts for this file.
func (sf *SiaFile) Redundancy(offlineMap map[string]bool, goodForRenewMap map[string]bool) float64 {
	sf.mu.RLock()
	defer sf.mu.RUnlock()
	if sf.metadata.fileSize == 0 {
		return -1
	}

	minPiecesRenew := ^uint64(0)
	minPiecesNoRenew := ^uint64(0)
	for _, chunk := range sf.chunks {
		// Loop over chunks and remember how many unique pieces of the chunk
		// were goodForRenew and how many were not.
		numPiecesRenew := uint64(0)
		numPiecesNoRenew := uint64(0)
		for _, pieceSet := range chunk.pieces {
			// Remember if we encountered a goodForRenew piece or a
			// !goodForRenew piece that was at least online.
			foundGoodForRenew := false
			foundOnline := false
			for _, piece := range pieceSet {
				offline, exists1 := offlineMap[string(piece.HostPubKey.Key)]
				goodForRenew, exists2 := goodForRenewMap[string(piece.HostPubKey.Key)]
				if exists1 != exists2 {
					build.Critical("contract can't be in one map but not in the other")
				}
				if !exists1 || offline {
					continue
				}
				// If we found a goodForRenew piece we can stop.
				if goodForRenew {
					foundGoodForRenew = true
					break
				}
				// Otherwise we continue since there might be other hosts with
				// the same piece that are goodForRenew. We still remember that
				// we found an online piece though.
				foundOnline = true
			}
			if foundGoodForRenew {
				numPiecesRenew++
				numPiecesNoRenew++
			} else if foundOnline {
				numPiecesNoRenew++
			}
		}
		// Remember the smallest number of goodForRenew pieces encountered.
		if numPiecesRenew < minPiecesRenew {
			minPiecesRenew = numPiecesRenew
		}
		// Remember the smallest number of !goodForRenew pieces encountered.
		if numPiecesNoRenew < minPiecesNoRenew {
			minPiecesNoRenew = numPiecesNoRenew
		}
	}

	// If the redundancy is smaller than 1x we return the redundancy that
	// includes contracts that are not good for renewal. The reason for this is
	// a better user experience. If the renter operates correctly, redundancy
	// should never go above numPieces / minPieces and redundancyNoRenew should
	// never go below 1.
	redundancy := float64(minPiecesRenew) / float64(sf.chunks[0].erasureCode.MinPieces())          // TODO this shouldn't be chunks[0]
	redundancyNoRenew := float64(minPiecesNoRenew) / float64(sf.chunks[0].erasureCode.MinPieces()) //TODO this shouldn't be chunks[0]
	if redundancy < 1 {
		return redundancyNoRenew
	}
	return redundancy
}

// UID returns a unique identifier for this file.
func (sf *SiaFile) UID() string {
	sf.mu.RLock()
	defer sf.mu.RUnlock()
	return sf.uid
}
