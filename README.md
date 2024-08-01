This is test repo demoing https://github.com/tetratelabs/wazero/issues/2294

There are 2 branches here with a identical benchmark:

* `main`: Multiple instances in one runtime with quickJS embedded (big wasm file).
* `multi`: One instance per runtime with quickJS dyncamically imported (small wasm file.)

I'm running benchmarks with `gobench` (`go install https://github.com/bep/gobench@latest`), and running:




```bash
gobench --base multi
```

Gives this on my MacBook:


````
name                          old time/op    new time/op    delta
KatexStartStop/PoolSize1-10     15.7ms ± 3%    16.5ms ± 3%     ~     (p=0.114 n=4+4)
KatexStartStop/PoolSize8-10     92.3ms ± 4%    20.1ms ± 2%  -78.23%  (p=0.029 n=4+4)
KatexStartStop/PoolSize16-10     178ms ± 2%      28ms ± 5%  -84.47%  (p=0.029 n=4+4)

name                          old alloc/op   new alloc/op   delta
KatexStartStop/PoolSize1-10     12.7MB ± 0%    11.3MB ± 0%  -11.06%  (p=0.029 n=4+4)
KatexStartStop/PoolSize8-10      102MB ± 0%      49MB ± 0%  -51.42%  (p=0.029 n=4+4)
KatexStartStop/PoolSize16-10     203MB ± 0%      93MB ± 0%  -54.30%  (p=0.029 n=4+4)

name                          old allocs/op  new allocs/op  delta
KatexStartStop/PoolSize1-10      19.8k ± 0%     25.8k ± 0%  +30.00%  (p=0.029 n=4+4)
KatexStartStop/PoolSize8-10       159k ± 0%       32k ± 0%  -79.61%  (p=0.029 n=4+4)
KatexStartStop/PoolSize16-10      317k ± 0%       40k ± 0%  -87.44%  (p=0.029 n=4+4)
```

The package `concurrentinstantiationexample` contains the concurrent example from [the wazero repo](https://github.com/tetratelabs/wazero/tree/main/examples/concurrent-instantiation) in a similar setup:


```bash
BenchmarkConcurrentInstantiation/OneRuntime/goroutines=10-10                5245            236092 ns/op          497181 B/op        528 allocs/op
BenchmarkConcurrentInstantiation/OneRuntime/goroutines=50-10                2149            551476 ns/op         1258045 B/op       1654 allocs/op
BenchmarkConcurrentInstantiation/OneRuntime/goroutines=100-10               1268            937423 ns/op         2208523 B/op       3059 allocs/op
BenchmarkConcurrentInstantiation/OneRuntime/goroutines=300-10                435           3019852 ns/op         6011552 B/op       8691 allocs/op
BenchmarkConcurrentInstantiation/ManyRuntime/goroutines=10-10               2733            380479 ns/op          919463 B/op       1060 allocs/op
BenchmarkConcurrentInstantiation/ManyRuntime/goroutines=50-10               1076           1124723 ns/op         2184652 B/op       3868 allocs/op
BenchmarkConcurrentInstantiation/ManyRuntime/goroutines=100-10               483           2556008 ns/op         5219776 B/op       8078 allocs/op
BenchmarkConcurrentInstantiation/ManyRuntime/goroutines=300-10               210           5813977 ns/op        11871761 B/op      22257 allocs/op
```