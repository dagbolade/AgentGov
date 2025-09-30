package policy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bytecodealliance/wasmtime-go/v3"
	"github.com/rs/zerolog/log"
)

type WASMLoader struct {
	engine *wasmtime.Engine
	config *wasmtime.Config
}

func NewWASMLoader() *WASMLoader {
	config := wasmtime.NewConfig()
	config.SetWasmMultiMemory(true)
	config.SetWasmThreads(false)

	return &WASMLoader{
		engine: wasmtime.NewEngineWithConfig(config),
		config: config,
	}
}

func (l *WASMLoader) LoadFromDir(dir string) (map[string]*WASMEvaluator, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}

	evaluators := make(map[string]*WASMEvaluator)

	for _, entry := range entries {
		if entry.IsDir() || !l.isWASMFile(entry.Name()) {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		eval, err := l.loadFile(path)
		if err != nil {
			log.Warn().Err(err).Str("file", entry.Name()).Msg("failed to load policy")
			continue
		}

		name := l.extractPolicyName(entry.Name())
		evaluators[name] = eval
	}

	if len(evaluators) == 0 {
		log.Warn().Str("dir", dir).Msg("no valid WASM policies found - all requests will be denied")
		return nil, fmt.Errorf("no WASM policies found in %s", dir)
	}

	return evaluators, nil
}

func (l *WASMLoader) loadFile(path string) (*WASMEvaluator, error) {
	wasmBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	module, err := wasmtime.NewModule(l.engine, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("compile module: %w", err)
	}

	return NewWASMEvaluator(l.engine, module)
}

func (l *WASMLoader) isWASMFile(filename string) bool {
	return strings.HasSuffix(strings.ToLower(filename), ".wasm")
}

func (l *WASMLoader) extractPolicyName(filename string) string {
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	return strings.ToLower(name)
}
