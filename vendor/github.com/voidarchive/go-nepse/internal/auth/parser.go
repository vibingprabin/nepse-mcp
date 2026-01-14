package auth

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// css.wasm contains obfuscated index-computation functions that mirror
// NEPSE's client-side token parsing logic. The API returns tokens with
// junk characters inserted at computed positions; these WASM functions
// determine which positions to strip based on the salt values.
//
//go:embed css.wasm
var cssWasm []byte

// tokenParser wraps a WASM runtime to compute token character indices.
// NEPSE obfuscates tokens by inserting characters at positions derived
// from 5 salt values. This parser replicates the browser's decoding logic.
type tokenParser struct {
	rt  wazero.Runtime
	cdx api.Function
	rdx api.Function
	bdx api.Function
	ndx api.Function
	mdx api.Function
}

func newTokenParser() (*tokenParser, error) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)

	compiled, err := rt.CompileModule(ctx, cssWasm)
	if err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("compile wasm: %w", err)
	}
	mod, err := rt.InstantiateModule(ctx, compiled, wazero.NewModuleConfig())
	if err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("instantiate wasm: %w", err)
	}

	exports := []string{"cdx", "rdx", "bdx", "ndx", "mdx"}
	funcs := make([]api.Function, len(exports))
	for i, name := range exports {
		f := mod.ExportedFunction(name)
		if f == nil {
			_ = rt.Close(ctx)
			return nil, fmt.Errorf("export %q not found", name)
		}
		funcs[i] = f
	}

	return &tokenParser{
		rt:  rt,
		cdx: funcs[0], rdx: funcs[1], bdx: funcs[2], ndx: funcs[3], mdx: funcs[4],
	}, nil
}

func (p *tokenParser) close() error {
	return p.rt.Close(context.Background())
}

// call5 invokes a WASM function with 5 integer arguments.
func (p *tokenParser) call5(f api.Function, a, b, c, d, e int) (int, error) {
	res, err := f.Call(context.Background(),
		uint64(uint32(a)), uint64(uint32(b)),
		uint64(uint32(c)), uint64(uint32(d)), uint64(uint32(e)),
	)
	if err != nil {
		return 0, err
	}
	return int(int32(res[0])), nil
}

type tokenIndices struct {
	access  []int
	refresh []int
}

// indicesFromSalts computes character positions to remove from obfuscated tokens.
// Each WASM function is called with a specific salt permutation - the ordering
// matches NEPSE's browser-side decoding logic exactly.
func (p *tokenParser) indicesFromSalts(s [5]int) (tokenIndices, error) {
	s1, s2, s3, s4, s5 := s[0], s[1], s[2], s[3], s[4]

	// Access token indices: each function uses a specific salt permutation
	accessCalls := []struct {
		fn            api.Function
		a, b, c, d, e int
	}{
		{p.cdx, s1, s2, s3, s4, s5},
		{p.rdx, s1, s2, s4, s3, s5},
		{p.bdx, s1, s2, s4, s3, s5},
		{p.ndx, s1, s2, s4, s3, s5},
		{p.mdx, s1, s2, s4, s3, s5},
	}

	access := make([]int, len(accessCalls))
	for i, call := range accessCalls {
		idx, err := p.call5(call.fn, call.a, call.b, call.c, call.d, call.e)
		if err != nil {
			return tokenIndices{}, err
		}
		access[i] = idx
	}

	// Refresh token indices: uses swapped s1/s2 and different permutations
	refreshCalls := []struct {
		fn            api.Function
		a, b, c, d, e int
	}{
		{p.cdx, s2, s1, s3, s5, s4},
		{p.rdx, s2, s1, s3, s4, s5},
		{p.bdx, s2, s1, s4, s3, s5},
		{p.ndx, s2, s1, s4, s3, s5},
		{p.mdx, s2, s1, s4, s3, s5},
	}

	refresh := make([]int, len(refreshCalls))
	for i, call := range refreshCalls {
		idx, err := p.call5(call.fn, call.a, call.b, call.c, call.d, call.e)
		if err != nil {
			return tokenIndices{}, err
		}
		refresh[i] = idx
	}

	return tokenIndices{access: access, refresh: refresh}, nil
}
