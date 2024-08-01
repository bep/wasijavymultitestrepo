package ext

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

//go:embed wasm/quickjs.wasm
var quickjsWasm []byte

type IDGetter interface {
	GetID() uint32
}

type Dispatcher[Q, R IDGetter] interface {
	Execute(ctx context.Context, q Q) (R, error)
	Close() error
}

type dispatcherPool[Q, R IDGetter] struct {
	counter     atomic.Uint32
	dispatchers []*dispatcher[Q, R]
}

// TODO1 unexport.
func (p *dispatcherPool[Q, R]) Get() *dispatcher[Q, R] {
	return p.dispatchers[int(p.counter.Add(1))%len(p.dispatchers)]
}

func (p *dispatcherPool[Q, R]) Close() error {
	for _, d := range p.dispatchers {
		if err := d.Close(); err != nil {
			return err
		}
	}
	return nil
}

type dispatcher[Q, R IDGetter] struct {
	zero R

	mu    sync.RWMutex
	encMu sync.Mutex

	pending map[uint32]*call[Q, R]

	stdin  ReadWriteCloser
	stdout ReadWriteCloser
	dec    *json.Decoder
	enc    *json.Encoder

	shutdown bool
	closing  bool

	close func() error
}

func (d *dispatcher[Q, R]) Close() error {
	return d.close()
}

var ErrShutdown = fmt.Errorf("dispatcher is shutting down")

var timerPool = sync.Pool{}

func getTimer(d time.Duration) *time.Timer {
	if v := timerPool.Get(); v != nil {
		timer := v.(*time.Timer)
		timer.Reset(d)
		return timer
	}
	return time.NewTimer(d)
}

func putTimer(t *time.Timer) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	timerPool.Put(t)
}

// Execute sends a request to the dispatcher and waits for the response.
func (p *dispatcherPool[Q, R]) Execute(ctx context.Context, q Q) (R, error) {
	d := p.Get()
	if q.GetID() == 0 {
		return d.zero, errors.New("ID must not be 0 (note that this must be unique within the current request set time window)")
	}

	call, err := d.newCall(q)
	if err != nil {
		return d.zero, err
	}

	if err := d.send(call); err != nil {
		return d.zero, err
	}

	timer := getTimer(30 * time.Second)
	defer putTimer(timer)

	select {
	case call = <-call.Done:
	case <-timer.C:
		return d.zero, errors.New("timeout")
	}

	if call.Error != nil {
		return d.zero, call.Error
	}

	return call.Response, nil
}

func (d *dispatcher[Q, R]) newCall(q Q) (*call[Q, R], error) {
	call := &call[Q, R]{
		Done:    make(chan *call[Q, R], 1),
		Request: q,
	}

	if d.shutdown || d.closing {
		call.Error = ErrShutdown
		call.done()
		return call, nil
	}

	d.mu.Lock()
	d.pending[q.GetID()] = call
	d.mu.Unlock()

	return call, nil
}

func (d *dispatcher[Q, R]) send(call *call[Q, R]) error {
	d.mu.RLock()
	if d.closing || d.shutdown {
		d.mu.RUnlock()
		return ErrShutdown
	}
	d.mu.RUnlock()

	d.encMu.Lock()
	defer d.encMu.Unlock()
	err := d.enc.Encode(call.Request)
	if err != nil {
		return err
	}
	return nil
}

func (d *dispatcher[Q, R]) input() {
	var inputErr error

	for d.dec.More() {
		var r R
		if err := d.dec.Decode(&r); err != nil {
			inputErr = err
			break
		}

		d.mu.Lock()
		call, found := d.pending[r.GetID()]
		if !found {
			d.mu.Unlock()
			panic(fmt.Errorf("call with ID %d not found", r.GetID()))
		}
		delete(d.pending, r.GetID())
		d.mu.Unlock()
		call.Response = r
		call.done()
	}

	// Terminate pending calls.
	d.shutdown = true
	if inputErr != nil {
		isEOF := inputErr == io.EOF || strings.Contains(inputErr.Error(), "already closed")
		if isEOF {
			if d.closing {
				inputErr = ErrShutdown
			} else {
				inputErr = io.ErrUnexpectedEOF
			}
		}
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	for _, call := range d.pending {
		call.Error = inputErr
		call.done()
	}
}

type call[Q, R any] struct {
	Request  Q
	Response R
	Error    error
	Done     chan *call[Q, R]
}

func (call *call[Q, R]) done() {
	select {
	case call.Done <- call:
	default:
	}
}

type Options struct {
	Ctx                  context.Context
	ModuleName           string
	ModuleSource         []byte
	NeedsQuickJSProvider bool
	CompilationCache     wazero.CompilationCache
	PoolSize             int
}

func Start[Q, R IDGetter](opts Options) (Dispatcher[Q, R], error) {
	if opts.PoolSize == 0 {
		opts.PoolSize = 1
	}

	if opts.Ctx == nil {
		opts.Ctx = context.Background()
	}

	pool := &dispatcherPool[Q, R]{
		dispatchers: make([]*dispatcher[Q, R], opts.PoolSize),
	}

	if opts.CompilationCache == nil {
		return nil, errors.New("CompilationCache is required")
	}

	runtimeConfig := wazero.NewRuntimeConfig().WithCompilationCache(opts.CompilationCache)
	// Compiled modules can be shared across runtimes.
	r := wazero.NewRuntimeWithConfig(opts.Ctx, runtimeConfig)

	var compiledQuickJS wazero.CompiledModule
	if opts.NeedsQuickJSProvider {
		var err error
		compiledQuickJS, err = r.CompileModule(opts.Ctx, quickjsWasm)
		if err != nil {
			return nil, err
		}
	}
	compiledMod, err := r.CompileModule(opts.Ctx, opts.ModuleSource)
	if err != nil {
		return nil, err
	}

	for i := 0; i < opts.PoolSize; i++ {
		d, err := newDispatcher[Q, R](opts, runtimeConfig, compiledMod, compiledQuickJS)
		if err != nil {
			return nil, err
		}
		pool.dispatchers[i] = d
	}
	return pool, nil
}

func newDispatcher[Q, R IDGetter](opts Options, runtimeConfig wazero.RuntimeConfig, mod, quickjs wazero.CompiledModule) (*dispatcher[Q, R], error) {
	if opts.Ctx == nil {
		opts.Ctx = context.Background()
	}

	ctx := opts.Ctx

	// Create a new WebAssembly Runtime.
	r := wazero.NewRuntimeWithConfig(opts.Ctx, runtimeConfig)

	// Instantiate WASI, which implements system I/O such as console output.
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, r); err != nil {
		return nil, err
	}

	var stdin, stdout ReadWriteCloser

	stdin = NewPipeReadWriteCloser()
	stdout = NewPipeReadWriteCloser()

	config := wazero.NewModuleConfig().WithStdout(stdout).WithStderr(os.Stderr).WithStdin(stdin)

	done := make(chan struct{})
	go func() {
		// This will block until stdin is closed.
		if _, err := r.InstantiateModule(ctx, quickjs, config.WithName("javy_quickjs_provider_v2")); err != nil {
			panic(err)
		}
		if _, err := r.InstantiateModule(ctx, mod, config.WithName(opts.ModuleName)); err != nil {
			panic(err)
		}
		close(done)
	}()

	var d *dispatcher[Q, R]

	close := func() error {
		d.closing = true

		if err := stdin.Close(); err != nil {
			return err
		}
		if err := stdout.Close(); err != nil {
			return err
		}

		// We need to wait for the WebAssembly instance to finish executing before we can close the runtime.
		<-done

		return r.Close(ctx)
	}

	d = &dispatcher[Q, R]{
		pending: make(map[uint32]*call[Q, R]),

		stdin:  stdin,
		stdout: stdout,
		dec:    json.NewDecoder(stdout),
		enc:    json.NewEncoder(stdin),

		close: close,
	}

	go d.input()

	return d, nil
}

func printStackTrace(w io.Writer) {
	buf := make([]byte, 1<<16)
	runtime.Stack(buf, true)
	fmt.Fprintf(w, "%s", buf)
}

func instanceFunc(name string, mod, quickjsMod wazero.CompiledModule) func(ctx context.Context, r wazero.Runtime, c wazero.ModuleConfig) (func() error, error) {
	return func(ctx context.Context, r wazero.Runtime, c wazero.ModuleConfig) (func() error, error) {
		return func() error {
			if _, err := r.InstantiateModule(ctx, quickjsMod, c.WithName("javy_quickjs_provider_v2")); err != nil {
				return err
			}
			if _, err := r.InstantiateModule(ctx, mod, c.WithName(name)); err != nil {
				return err
			}
			return nil
		}, nil
	}
}

func compileFunc(name string, wasm []byte, needsQuickJSProvider bool) func(ctx context.Context, r wazero.Runtime, c wazero.ModuleConfig) (func() error, error) {
	return func(ctx context.Context, r wazero.Runtime, c wazero.ModuleConfig) (func() error, error) {
		var compiledQuickJS wazero.CompiledModule
		if needsQuickJSProvider {
			var err error
			compiledQuickJS, err = r.CompileModule(ctx, quickjsWasm)
			if err != nil {
				return nil, err
			}
		}
		compileGreet, err := r.CompileModule(ctx, wasm)
		if err != nil {
			return nil, err
		}

		return func() error {
			if needsQuickJSProvider {
				// Instansiate the QuickJS module.
				if _, err := r.InstantiateModule(ctx, compiledQuickJS, c.WithName("javy_quickjs_provider_v2")); err != nil {
					return err
				}
			}
			if _, err := r.InstantiateModule(ctx, compileGreet, c.WithName(name)); err != nil {
				return err
			}
			return nil
		}, nil
	}
}

// TODO1 notes
/*

QuickJS native JSON intrinsic https://github.com/bytecodealliance/javy/blob/main/crates/javy/src/config.rs
Whether to override the implementation of JSON.parse and JSON.stringify
*/

type ReadWriteCloser interface {
	io.Reader
	io.Writer
	io.Closer
}

// PipeReadWriteCloser is a convenience type to create a pipe with a ReadCloser and a WriteCloser.
type PipeReadWriteCloser struct {
	*io.PipeReader
	*io.PipeWriter
}

// NewPipeReadWriteCloser creates a new PipeReadWriteCloser.
func NewPipeReadWriteCloser() PipeReadWriteCloser {
	pr, pw := io.Pipe()
	return PipeReadWriteCloser{pr, pw}
}

func (c PipeReadWriteCloser) Close() (err error) {
	if err = c.PipeReader.Close(); err != nil {
		return
	}
	err = c.PipeWriter.Close()
	return
}

func (c PipeReadWriteCloser) WriteString(s string) (int, error) {
	return c.PipeWriter.Write([]byte(s))
}
