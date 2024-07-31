package ext

import (
	"context"
	_ "embed"
	"fmt"
	"testing"

	"github.com/tetratelabs/wazero"
)

//go:embed wasm/greet.wasm
var greetWasm []byte

type person struct {
	ID   uint32 `json:"id"`
	Name string `json:"name"`
}

func (p person) GetID() uint32 {
	return p.ID
}

func TestGreet(t *testing.T) {
	opts := Options{
		CompileModule: compileFunc("greet", greetWasm, true),
	}

	d, err := Start[person, greeting](opts)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	ctx := context.Background()

	inputPerson := person{
		Name: "Person",
	}

	for i := 0; i < 10; i++ {
		inputPerson.ID = uint32(i + 1)
		g, err := d.Execute(ctx, inputPerson)
		if err != nil {
			t.Fatal(err)
		}
		if g.Greeting != "Hello Person!" {
			t.Fatalf("got: %v", g)
		}
		if g.ID != inputPerson.ID {
			t.Fatalf("%d vs %d", g.ID, inputPerson.ID)
		}
	}
}

func BenchmarkKatexStartStop(b *testing.B) {
	tempDir := b.TempDir()

	compilationCache, err := wazero.NewCompilationCacheWithDir(tempDir)
	if err != nil {
		b.Fatal(err)
	}

	optsTemplate := Options{
		CompileModule:    compileFunc("katex", katexWasm, true),
		CompilationCache: compilationCache,
	}

	runBench := func(b *testing.B, opts Options) {
		for i := 0; i < b.N; i++ {
			d, err := Start[KatexInput, KatexOutput](opts)
			if err != nil {
				b.Fatal(err)
			}
			if err := d.Close(); err != nil {
				b.Fatal(err)
			}
		}
	}

	for _, poolSize := range []int{1, 8, 16} {

		name := fmt.Sprintf("PoolSize%d", poolSize)

		b.Run(name, func(b *testing.B) {
			opts := optsTemplate
			opts.PoolSize = poolSize
			runBench(b, opts)
		})

	}
}

var katexInputTemplate = KatexInput{
	Expression:  "c = \\pm\\sqrt{a^2 + b^2}",
	DisplayMode: true,
	Output:      "html",
}

type greeting struct {
	ID       uint32 `json:"id"`
	Greeting string `json:"greeting"`
}

func (g greeting) GetID() uint32 {
	return g.ID
}
