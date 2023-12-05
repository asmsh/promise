package uniquerand

import (
	"testing"
)

var uniquerandIntTestCases = []struct {
	name string
	n    int
}{
	{
		name: "default",
		n:    -1,
	},
	{
		name: "range 32",
		n:    32,
	},
	{
		name: "range 64",
		n:    64,
	},
	{
		name: "range 256",
		n:    256,
	},
	{
		name: "range 1024",
		n:    1024,
	},
	{
		name: "range 4096",
		n:    4096,
	},
}

func Test_Int_Get(t *testing.T) {
	for _, tt := range uniquerandIntTestCases {
		t.Run(tt.name, func(t *testing.T) {
			uniqueMap := map[int]struct{}{}

			uri := Int{}
			uri.Reset(tt.n)

			// consume the source while making sure of uniqueness
			for urn, ok := uri.Get(); ok; urn, ok = uri.Get() {
				if _, duplicate := uniqueMap[urn]; duplicate {
					t.Errorf("Get() duplicate number = %v", urn)
				}
				uniqueMap[urn] = struct{}{}
			}

			// make sure we produced all the range
			if gotN := len(uniqueMap); gotN != defRange && gotN != tt.n {
				t.Errorf("Get() produced less numbers = %v (%v)", gotN, uniqueMap)
			}
		})
	}
}

func OK[T any](f func() (T, bool)) bool {
	_, ok := f()
	return ok
}

func Test_Int_Put(t *testing.T) {
	for _, tt := range uniquerandIntTestCases {
		t.Run(tt.name, func(t *testing.T) {
			if tt.n <= 0 {
				t.SkipNow()
			}

			uri := Int{}
			uri.Reset(tt.n)

			// putting any number in an empty Int should return false
			for i := 0; i < tt.n; i++ {
				if gotOk := uri.Put(i); gotOk {
					t.Errorf("Put() invalid unique number = %v", i)
				}
			}

			// consume all numbers in the Int
			for OK(uri.Get) {
			}

			// putting the numbers back in a consumed Int should return true
			for i := 0; i < tt.n; i++ {
				if gotOk := uri.Put(i); !gotOk {
					t.Errorf("Put() unexpected unique number = %v", i)
				}
			}

			// putting any number in an empty Int should return false
			for i := 0; i < tt.n; i++ {
				if gotOk := uri.Put(i); gotOk {
					t.Errorf("Put() duplicate number = %v", i)
				}
			}
		})
	}
}

func Benchmark_Int(b *testing.B) {
	for _, bm := range uniquerandIntTestCases {
		b.Run(bm.name, func(b *testing.B) {
			b.Run("Get", func(b *testing.B) {
				uri := Int{}
				uri.Reset(bm.n)

				b.ReportAllocs()

				for i := 0; i < b.N; i++ {
					uri.Get()
				}
			})

			b.Run("Get & Put", func(b *testing.B) {
				uri := Int{}
				uri.Reset(bm.n)

				b.ReportAllocs()

				for i := 0; i < b.N; i++ {
					n, _ := uri.Get()
					uri.Put(n)
				}
			})

			b.Run("Rest & Get", func(b *testing.B) {
				b.ReportAllocs()

				for i := 0; i < b.N; i++ {
					uri := Int{}
					uri.Reset(bm.n)
					uri.Get()
				}
			})
		})
	}
}
