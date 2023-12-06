/*
Package ed2k implements an ed2k hasher, as explained in
http://wiki.anidb.net/w/Ed2k-hash#How_is_an_ed2k_hash_calculated_exactly.3F.

The ed2k hash is essentially a Merkle tree of depth 1 and fixed BlockSize.
Each leaf node's hash is calculated on its own goroutine.
Calling Sum() will wait for the hashing goroutines.
*/
package ed2k

import (
	"encoding/hex"
	"hash"
	"io"
	"runtime"

	"golang.org/x/crypto/md4"
)

// The size of the ed2k checksum in bytes.
const Size = md4.Size

// The size of each hash node in bytes.
const BlockSize = 9728000

// A hash.Hash that also needs to be Close()d when done.
type HashCloser interface {
	hash.Hash
	io.Closer
}

type digest struct {
	currentChunk     []byte
	endWithNullChunk bool

	reqCurrentHashes chan bool
	currentHashes    chan []byte
	addHash          chan chan []byte
	quitLoop         chan bool
}

func (d *digest) hashLoop() {
	var (
		notify        bool
		hashList      = make([]byte, 0)
		runningHashes = make([]chan []byte, 0)
		maxProcs      = runtime.GOMAXPROCS(0)
	)
	for {
		var nextHash <-chan []byte

		addHash := d.addHash
		if l := len(runningHashes); l > 0 {
			// make Write() block if we already have 2*GOMAXPROCS hashes flying
			// avoids having to keep too many live block slices around, specially since they're almost 10MB each
			if l >= 2*maxProcs {
				addHash = nil
			}
			nextHash = runningHashes[0]
		} else if notify {
			notify = false
			list := make([]byte, len(hashList))
			copy(list, hashList)
			d.currentHashes <- list
		}

		select {
		case c := <-addHash:
			runningHashes = append(runningHashes, c)
		case hash := <-nextHash:
			hashList = append(hashList, hash...)
			runningHashes = runningHashes[1:]
		case <-d.reqCurrentHashes:
			notify = true
		case <-d.quitLoop:
			// flush the running hashes
			go func() {
				for _, c := range runningHashes {
					<-c
				}
			}()
			if notify {
				d.currentHashes <- []byte{}
			}
			d.quitLoop <- true
			return
		}
	}
}

func (d *digest) Reset() {
	d.currentChunk = make([]byte, 0, BlockSize)

	if d.quitLoop != nil {
		d.quitLoop <- true
		<-d.quitLoop
	} else {
		d.reqCurrentHashes = make(chan bool)
		d.quitLoop = make(chan bool)
		d.currentHashes = make(chan []byte)
		d.addHash = make(chan chan []byte)
	}

	go d.hashLoop()
}

// Stops the background hasher and releases all memory
// used by chunks.
//
// The hash can be used again if it's Reset().
func (d *digest) Close() error {
	if d.quitLoop != nil {
		d.quitLoop <- true
		<-d.quitLoop
		d.quitLoop = nil

		d.currentChunk = nil
	}
	return nil
}

// New returns a new hash.Hash computing the ed2k checksum.
//
// The bool argument chooses between the new (false) or old (true) blockchain finishing algorithm.
// The old algorithm was due to an off-by-one bug in the de facto implementation, but is still used
// in some cases.
//
// In the page given in the package description, false picks the "blue" method, true picks the "red" method.
//
// See hash.Hash for the interface.
func New(endWithNullChunk bool) HashCloser {
	d := &digest{endWithNullChunk: endWithNullChunk}
	d.Reset()
	return d
}

func (d *digest) Size() int      { return Size }
func (d *digest) BlockSize() int { return BlockSize }

func (d *digest) Write(p []byte) (i int, err error) {
	for i = 0; i < len(p); {
		count := copy(d.currentChunk[len(d.currentChunk):cap(d.currentChunk)], p[i:])
		d.currentChunk = d.currentChunk[:len(d.currentChunk)+count]
		i += count
		if len(d.currentChunk) == cap(d.currentChunk) && len(p[i:]) > 0 {
			d.addHash <- md4SumAsync(d.currentChunk)
			// the old currentChunk now belongs to the md4 goroutine, make a new one
			d.currentChunk = make([]byte, 0, BlockSize)
		}
	}
	return
}

func (d *digest) Sum(p []byte) []byte {
	currentChunk := d.currentChunk

	d.reqCurrentHashes <- true
	hashList := <-d.currentHashes

	if d.endWithNullChunk && len(currentChunk) == cap(currentChunk) {
		hashList = md4Sum(currentChunk, hashList)
		currentChunk = currentChunk[:0] // Leave a null chunk for appending
	} else if len(hashList) == 0 {
		// We just hash the data itself, instead of "chunking"
		return md4Sum(currentChunk, nil)
	}
	// We always append a chunk if d.endWithNullChunk, regardless of length
	if d.endWithNullChunk || len(currentChunk) > 0 {
		hashList = md4Sum(currentChunk, hashList)
	}

	return md4Sum(hashList, p)
}

func (d *digest) String() string {
	return hex.EncodeToString(d.Sum(nil))
}

func md4Sum(data []byte, list []byte) []byte {
	md4 := md4.New()
	md4.Write(data)
	return md4.Sum(list)
}

func md4SumAsync(data []byte) chan []byte {
	c := make(chan []byte)
	go func() {
		c <- md4Sum(data, nil)
	}()
	return c
}
