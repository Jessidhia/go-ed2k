/* Package ed2k implements an ed2k hasher as explained in http://wiki.anidb.net/w/Ed2k-hash#How_is_an_ed2k_hash_calculated_exactly.3F */
package ed2k

import (
	"code.google.com/p/go.crypto/md4"
	"fmt"
	"hash"
	"strings"
)

// The size of the ed2k checksum in bytes
const Size = md4.Size

// The chunk size in bytes
const BlockSize = 9728000

type digest struct {
	currentChunk     []byte
	hashList         []byte
	endWithNullChunk bool
	md4              hash.Hash
}

func (d *digest) Reset() {
	d.currentChunk = make([]byte, 0, BlockSize)
	d.hashList = make([]byte, 0, Size) // hashList can grow arbitrarily; we just give it an initial non-tiny capacity
	if d.md4 == nil {
		d.md4 = md4.New()
	} else {
		d.md4.Reset()
	}
}

// New returns a new hash.Hash computing the ed2k checksum.
// The bool argument chooses between the new (false) or old (true) blockchain finishing algorithm.
// In the page given in the package description, false picks the "blue" method, true picks the "red" method.
func New(endWithNullChunk bool) hash.Hash {
	d := new(digest)
	d.endWithNullChunk = endWithNullChunk
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
			d.hashList = d.md4sum(d.currentChunk, d.hashList)
			d.currentChunk = d.currentChunk[:0] // reset len to 0
		}
	}
	return
}

func (d0 *digest) Sum(p []byte) []byte {
	d := new(digest)
	*d = *d0

	if d.endWithNullChunk && len(d.currentChunk) == cap(d.currentChunk) {
		d.hashList = d.md4sum(d.currentChunk, d.hashList)
		d.currentChunk = d.currentChunk[:0] // Leave a null chunk for appending
	} else if len(d.hashList) == 0 {
		// We just hash the data itself, instead of "chunking"
		ret := d.md4sum(d.currentChunk, nil)
		d.currentChunk = nil
		return ret
	}
	// We always append a chunk if d.endWithNullChunk, regardless of length
	if d.endWithNullChunk || len(d.currentChunk) > 0 {
		d.hashList = d.md4sum(d.currentChunk, d.hashList)
	}

	d.currentChunk = nil // release memory
	return d.md4sum(d.hashList, p)
}

// Returns an hexadecimal representation of the hash
func (d *digest) String() string {
	sum := d.Sum(nil)
	parts := make([]string, 0, 2*Size)
	for _, b := range sum {
		parts = append(parts, fmt.Sprintf("%02x", b))
	}
	return strings.Join(parts, "")
}

func (d *digest) md4sum(data []byte, list []byte) []byte {
	d.md4.Reset()
	d.md4.Write(data)
	return d.md4.Sum(list)
}
