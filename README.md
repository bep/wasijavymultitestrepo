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
