package ed2k_test

import (
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/Jessidhia/go-ed2k"
)

// A "fake" reader that never writes anything to the []byte.
// Effectively always reads len(p) of NULs.
type FakeReader struct{}

func (_ *FakeReader) Read(p []byte) (n int, err error) {
	return len(p), nil
}

type testVector struct {
	Mode bool
	Data io.Reader
	Size int64
	Hash string
}

func (vec *testVector) String() string {
	mode := "without"
	if vec.Mode == true {
		mode = "with"
	}
	return fmt.Sprintf("size %d, expected hash %#v %s nullchunk", vec.Size, vec.Hash, mode)
}

const chunkSize = ed2k.BlockSize

var fakeReader = &FakeReader{}
var vectors = []*testVector{
	{Mode: false, Data: nil, Size: 0, Hash: "31d6cfe0d16ae931b73c59d7e0c089c0"},
	{Mode: false, Data: strings.NewReader("small example"), Size: -1, Hash: "3e01197bc54364cb86a41738b06ae679"},
	{Mode: true, Data: fakeReader, Size: chunkSize, Hash: "fc21d9af828f92a8df64beac3357425d"},
	{Mode: true, Data: fakeReader, Size: 2 * chunkSize, Hash: "114b21c63a74b6ca922291a11177dd5c"},
	{Mode: false, Data: fakeReader, Size: chunkSize, Hash: "d7def262a127cd79096a108e7a9fc138"},
	{Mode: false, Data: fakeReader, Size: 2 * chunkSize, Hash: "194ee9e4fa79b2ee9f8829284c466051"},
}

func Test(T *testing.T) {
	T.Parallel()
	for i, vec := range vectors {
		ed2k := ed2k.New(vec.Mode)
		if vec.Size == 0 { // do nothing
		} else if vec.Size < 0 {
			io.Copy(ed2k, vec.Data)
		} else {
			io.CopyN(ed2k, vec.Data, vec.Size)
		}
		if ed2k.(fmt.Stringer).String() != vec.Hash {
			T.Errorf("Vector #%d %v did not match expected hash %#v", i, vec, vec.Hash)
		}
	}
}

func Example_hexString() {
	e := ed2k.New(false)
	io.Copy(e, strings.NewReader("small example"))
	fmt.Println(hex.EncodeToString(e.Sum(nil)))

	// for convenience, ed2k implements Stringer by doing just that
	fmt.Println(e)
	// Output:
	// 3e01197bc54364cb86a41738b06ae679
	// 3e01197bc54364cb86a41738b06ae679
}

func Example_noNullChunk() {
	e := ed2k.New(false)
	io.Copy(e, strings.NewReader("small example"))
	h := e.Sum(nil)
	fmt.Println(h)
	// Output: [62 1 25 123 197 67 100 203 134 164 23 56 176 106 230 121]
}

func bench(B *testing.B, mode bool, size int64) {
	B.SetBytes(size)

	ed2k := ed2k.New(mode)
	B.ResetTimer()
	for i := 0; i < B.N; i++ {
		ed2k.Reset()
		io.CopyN(ed2k, fakeReader, size)
		ed2k.Sum(nil)
	}
}

func Benchmark_nullChunk(B *testing.B) {
	bench(B, true, chunkSize)
}

func Benchmark_noNullChunk(B *testing.B) {
	bench(B, false, chunkSize)
}

func Benchmark_1MB(B *testing.B) {
	bench(B, false, 1*1024*1024)
}

func Benchmark_10MB(B *testing.B) {
	bench(B, false, 10*1024*1024)
}

func Benchmark_100MB(B *testing.B) {
	bench(B, false, 100*1024*1024)
}

func Benchmark_1GB(B *testing.B) {
	bench(B, false, 1*1024*1024*1024)
}

func Benchmark_10GB(B *testing.B) {
	bench(B, false, 10*1024*1024*1024)
}
