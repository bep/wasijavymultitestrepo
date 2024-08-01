package concurrentinstantiationexample

import (
	"fmt"
	"testing"
)

func TestRunOneRuntime(t *testing.T) {
	OneRuntime(50)
}

func TestRunOneRuntimePerInstance(t *testing.T) {
	OneRuntimePerInstance(50)
}

func BenchmarkConcurrentInstantiation(b *testing.B) {
	runBenchmark := func(b *testing.B, f func(int)) {
		for _, i := range []int{10, 50, 100, 300} {
			b.Run(fmt.Sprintf("instances=%d", i), func(b *testing.B) {
				for j := 0; j < b.N; j++ {
					f(i)
				}
			})
		}
	}

	b.Run("OneRuntime", func(b *testing.B) {
		runBenchmark(b, OneRuntime)
	})

	b.Run("ManyRuntime", func(b *testing.B) {
		runBenchmark(b, OneRuntimePerInstance)
	})
}
