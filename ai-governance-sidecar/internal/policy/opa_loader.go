
package policy

import (
	"context"
	"github.com/open-policy-agent/opa/v1/rego"
)

type OPALoader struct{}

type OPAEvaluator struct {
	policyPath string
}

func NewOPALoader() *OPALoader {
	return &OPALoader{}
}

func (l *OPALoader) LoadFromFile(path string) (*OPAEvaluator, error) {
       // Just store the path; we'll load and evaluate with rego at runtime
       return &OPAEvaluator{policyPath: path}, nil
}

func (e *OPAEvaluator) Eval(ctx context.Context, input map[string]interface{}) (bool, error) {
       r := rego.New(
	       rego.Query("data.allow"),
	       rego.Load([]string{e.policyPath}, nil),
	       rego.Input(input),
       )
       rs, err := r.Eval(ctx)
       if err != nil {
	       return false, err
       }
       if len(rs) == 0 || len(rs[0].Expressions) == 0 {
	       return false, nil
       }
       allow, ok := rs[0].Expressions[0].Value.(bool)
       return ok && allow, nil
}

func (e *OPAEvaluator) Close() error {
	return nil
}
