package policy

import (
	"context"
	"fmt"
	"sync"

	"github.com/rs/zerolog/log"
)

type Engine struct {
	mu         sync.RWMutex
	loader     *WASMLoader
	watcher    *FileWatcher
	evaluators map[string]*WASMEvaluator
}

func NewEngine(policyDir string) (*Engine, error) {
	loader := NewWASMLoader()
	
	engine := &Engine{
		loader:     loader,
		evaluators: make(map[string]*WASMEvaluator),
	}

	if err := engine.loadPolicies(policyDir); err != nil {
		return nil, fmt.Errorf("initial load: %w", err)
	}

	watcher, err := NewFileWatcher(policyDir, engine.handlePolicyChange)
	if err != nil {
		return nil, fmt.Errorf("create watcher: %w", err)
	}
	engine.watcher = watcher

	return engine, nil
}

func (e *Engine) Evaluate(ctx context.Context, req Request) (Response, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if len(e.evaluators) == 0 {
		return e.denyResponse("no policies loaded"), nil
	}

	// Evaluate all policies; deny if any denies
	for name, eval := range e.evaluators {
		resp, err := eval.Evaluate(ctx, req)
		if err != nil {
			log.Warn().Err(err).Str("policy", name).Msg("policy evaluation failed")
			return e.denyResponse(fmt.Sprintf("policy error: %s", name)), nil
		}

		if !resp.Allow {
			return resp, nil
		}

		if resp.HumanRequired {
			return resp, nil
		}
	}

	return Response{Allow: true, Reason: "all policies passed"}, nil
}

func (e *Engine) Reload() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.reloadLocked()
}

func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.watcher != nil {
		if err := e.watcher.Close(); err != nil {
			return err
		}
	}

	for _, eval := range e.evaluators {
		if err := eval.Close(); err != nil {
			log.Warn().Err(err).Msg("failed to close evaluator")
		}
	}

	return nil
}

func (e *Engine) loadPolicies(dir string) error {
	policies, err := e.loader.LoadFromDir(dir)
	if err != nil {
		return err
	}

	for name, eval := range policies {
		e.evaluators[name] = eval
		log.Info().Str("policy", name).Msg("policy loaded")
	}

	return nil
}

func (e *Engine) reloadLocked() error {
	// Close existing evaluators
	for _, eval := range e.evaluators {
		eval.Close()
	}
	e.evaluators = make(map[string]*WASMEvaluator)

	// Reload from directory
	policies, err := e.loader.LoadFromDir(e.watcher.dir)
	if err != nil {
		return err
	}

	for name, eval := range policies {
		e.evaluators[name] = eval
	}

	log.Info().Int("count", len(policies)).Msg("policies reloaded")
	return nil
}

func (e *Engine) handlePolicyChange(path string) {
	log.Info().Str("path", path).Msg("policy change detected")
	
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.reloadLocked(); err != nil {
		log.Error().Err(err).Msg("failed to reload policies")
	}
}

func (e *Engine) denyResponse(reason string) Response {
	return Response{
		Allow:  false,
		Reason: reason,
	}
}