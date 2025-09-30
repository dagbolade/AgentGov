package policy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
)

type Engine struct {
       mu         sync.RWMutex
       loader     *OPALoader
       watcher    *FileWatcher
       evaluators map[string]*OPAEvaluator
}

func NewEngine(policyDir string) (*Engine, error) {
       loader := NewOPALoader()

       engine := &Engine{
	       loader:     loader,
	       evaluators: make(map[string]*OPAEvaluator),
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
	       // Convert Request to map[string]interface{} for OPA
	       input := map[string]interface{}{
		       "tool_name": req.ToolName,
		       "args":      req.Args,
		       "metadata":  req.Metadata,
	       }
	       allowed, err := eval.Eval(ctx, input)
	       if err != nil {
		       log.Warn().Err(err).Str("policy", name).Msg("policy evaluation failed")
		       return e.denyResponse(fmt.Sprintf("policy error: %s", name)), nil
	       }
	       if !allowed {
		       return Response{Allow: false, Reason: "denied by policy: " + name}, nil
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
       entries, err := os.ReadDir(dir)
       if err != nil {
	       return err
       }

       for _, entry := range entries {
	       if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".rego") {
		       continue
	       }
	       path := filepath.Join(dir, entry.Name())
	       eval, err := e.loader.LoadFromFile(path)
	       if err != nil {
		       log.Warn().Err(err).Str("file", entry.Name()).Msg("failed to load policy")
		       continue
	       }
	       name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
	       e.evaluators[name] = eval
	       log.Info().Str("policy", name).Msg("policy loaded")
       }
       if len(e.evaluators) == 0 {
	       log.Warn().Str("dir", dir).Msg("no valid OPA policies found - all requests will be denied")
       }
       return nil
}

func (e *Engine) reloadLocked() error {
       e.evaluators = make(map[string]*OPAEvaluator)
       return e.loadPolicies(e.watcher.dir)
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