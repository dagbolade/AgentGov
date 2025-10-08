package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	wasmtime "github.com/bytecodealliance/wasmtime-go/v3"
)

type WASMEvaluator struct {
	store    *wasmtime.Store
	instance *wasmtime.Instance
	memory   *wasmtime.Memory
	evaluate *wasmtime.Func
}

func NewWASMEvaluator(engine *wasmtime.Engine, module *wasmtime.Module) (*WASMEvaluator, error) {
	store := wasmtime.NewStore(engine)
	// Add fuel to prevent "all fuel consumed" errors
	store.AddFuel(10000000) // 10 million units should be plenty
	linker := wasmtime.NewLinker(engine)

	eval := &WASMEvaluator{store: store}

	if err := eval.defineHostFunctions(linker); err != nil {
		return nil, fmt.Errorf("define host functions: %w", err)
	}

	instance, err := linker.Instantiate(store, module)
	if err != nil {
		return nil, fmt.Errorf("instantiate: %w", err)
	}
	eval.instance = instance

	if err := eval.bindExports(); err != nil {
		return nil, err
	}

	return eval, nil
}

func (e *WASMEvaluator) Evaluate(ctx context.Context, req Request) (Response, error) {
	inputJSON, err := json.Marshal(req)
	if err != nil {
		return Response{}, fmt.Errorf("marshal request: %w", err)
	}

	outputJSON, err := e.callEvaluate(inputJSON)
	if err != nil {
		return Response{}, err
	}

	var resp Response
	if err := json.Unmarshal(outputJSON, &resp); err != nil {
		return Response{}, fmt.Errorf("unmarshal response: %w", err)
	}

	return resp, nil
}

func (e *WASMEvaluator) Close() error {
	return nil
}

func (e *WASMEvaluator) callEvaluate(input []byte) ([]byte, error) {
	inputPtr, err := e.allocateMemory(len(input))
	if err != nil {
		return nil, fmt.Errorf("allocate input: %w", err)
	}

	if err := e.writeMemory(inputPtr, input); err != nil {
		return nil, fmt.Errorf("write input: %w", err)
	}

	outputPtr, err := e.allocateMemory(8192)
	if err != nil {
		return nil, fmt.Errorf("allocate output: %w", err)
	}

	result, err := e.evaluate.Call(e.store, inputPtr, len(input), outputPtr, 8192)
	if err != nil {
		return nil, fmt.Errorf("call evaluate: %w", err)
	}

	if result.(int32) != 0 {
		return nil, fmt.Errorf("evaluation failed with code %d", result)
	}

	return e.readMemory(outputPtr, 8192), nil
}

func (e *WASMEvaluator) defineHostFunctions(linker *wasmtime.Linker) error {
	// Define log function: (ptr: i32, len: i32) -> void
	logType := wasmtime.NewFuncType(
		[]*wasmtime.ValType{
			wasmtime.NewValType(wasmtime.KindI32),
			wasmtime.NewValType(wasmtime.KindI32),
		},
		[]*wasmtime.ValType{},
	)

	if err := linker.FuncNew("env", "log", logType, e.hostLog); err != nil {
		return err
	}

	// Define get_env function: (key_ptr: i32, key_len: i32, out_ptr: i32, out_max_len: i32) -> i32
	getEnvType := wasmtime.NewFuncType(
		[]*wasmtime.ValType{
			wasmtime.NewValType(wasmtime.KindI32),
			wasmtime.NewValType(wasmtime.KindI32),
			wasmtime.NewValType(wasmtime.KindI32),
			wasmtime.NewValType(wasmtime.KindI32),
		},
		[]*wasmtime.ValType{
			wasmtime.NewValType(wasmtime.KindI32),
		},
	)

	if err := linker.FuncNew("env", "get_env", getEnvType, e.hostGetEnv); err != nil {
		return err
	}

	return nil
}

func (e *WASMEvaluator) bindExports() error {
	memExport := e.instance.GetExport(e.store, "memory")
	if memExport == nil {
		return fmt.Errorf("memory export not found")
	}
	e.memory = memExport.Memory()

	evalExport := e.instance.GetExport(e.store, "evaluate")
	if evalExport == nil {
		return fmt.Errorf("evaluate export not found")
	}
	e.evaluate = evalExport.Func()

	return nil
}

func (e *WASMEvaluator) allocateMemory(size int) (int32, error) {
	allocExport := e.instance.GetExport(e.store, "allocate")
	if allocExport == nil {
		return 0, fmt.Errorf("allocate export not found")
	}

	result, err := allocExport.Func().Call(e.store, size)
	if err != nil {
		return 0, err
	}

	return result.(int32), nil
}

func (e *WASMEvaluator) writeMemory(ptr int32, data []byte) error {
	mem := e.memory.UnsafeData(e.store)
	copy(mem[ptr:], data)
	return nil
}

func (e *WASMEvaluator) readMemory(ptr int32, maxLen int) []byte {
	mem := e.memory.UnsafeData(e.store)

	end := ptr
	for i := 0; i < maxLen; i++ {
		if mem[ptr+int32(i)] == 0 {
			break
		}
		end++
	}

	return mem[ptr:end]
}

func (e *WASMEvaluator) hostLog(caller *wasmtime.Caller, args []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
	msgPtr := args[0].I32()
	msgLen := args[1].I32()

	mem := caller.GetExport("memory").Memory().UnsafeData(caller)
	msg := string(mem[msgPtr : msgPtr+msgLen])
	fmt.Printf("[WASM] %s\n", msg)

	return []wasmtime.Val{}, nil
}

func (e *WASMEvaluator) hostGetEnv(caller *wasmtime.Caller, args []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
	keyPtr := args[0].I32()
	keyLen := args[1].I32()
	outPtr := args[2].I32()
	outMaxLen := args[3].I32()

	mem := caller.GetExport("memory").Memory().UnsafeData(caller)
	key := string(mem[keyPtr : keyPtr+keyLen])

	value := os.Getenv(key)
	if value == "" {
		return []wasmtime.Val{wasmtime.ValI32(-1)}, nil
	}

	valueBytes := []byte(value)
	if len(valueBytes) > int(outMaxLen) {
		return []wasmtime.Val{wasmtime.ValI32(-1)}, nil
	}

	copy(mem[outPtr:], valueBytes)
	return []wasmtime.Val{wasmtime.ValI32(int32(len(valueBytes)))}, nil
}
