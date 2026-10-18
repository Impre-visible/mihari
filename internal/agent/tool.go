// Package agent runs the loop where a local model works on a job by calling
// tools, until it calls finish or runs out of turns or time.
//
// Adding a tool is one call to Register(Define(...)) in any file of this
// package: the JSON schema the model sees comes from the argument struct.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// Tool is something the model can call.
type Tool interface {
	Name() string
	Description() string
	// JSON schema of the arguments.
	Schema() map[string]any
	// Whether the tool is offered for this job: a push tool, say, only where pushing makes sense.
	Available(env *Env) bool
	Run(ctx context.Context, env *Env, args json.RawMessage) (string, error)
}

var (
	registryMu sync.Mutex
	registry   []Tool
)

// Register adds tools to every job they are available for.
func Register(tools ...Tool) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = append(registry, tools...)
}

// toolsFor returns the registered tools available for env.
func toolsFor(env *Env) []Tool {
	registryMu.Lock()
	defer registryMu.Unlock()
	var out []Tool
	for _, t := range registry {
		if t.Available(env) {
			out = append(out, t)
		}
	}
	return out
}

type typedTool[A any] struct {
	name, description string
	schema            map[string]any
	run               func(context.Context, *Env, A) (string, error)
	available         func(*Env) bool
}

func (t *typedTool[A]) Name() string           { return t.name }
func (t *typedTool[A]) Description() string    { return t.description }
func (t *typedTool[A]) Schema() map[string]any { return t.schema }

func (t *typedTool[A]) Available(env *Env) bool {
	return t.available == nil || t.available(env)
}

func (t *typedTool[A]) Run(ctx context.Context, env *Env, raw json.RawMessage) (string, error) {
	var args A
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("arguments invalides pour %s : %w", t.name, err)
		}
	}
	return t.run(ctx, env, args)
}

// Option tweaks a tool made with Define.
type Option func(*toolOptions)

type toolOptions struct {
	available func(*Env) bool
}

// When offers the tool only for jobs where ok returns true.
func When(ok func(*Env) bool) Option {
	return func(o *toolOptions) { o.available = ok }
}

// Define makes a tool from a function taking its arguments as a struct A.
// Each field's json tag names the argument, `desc` describes it to the model,
// and omitempty makes it optional.
func Define[A any](name, description string, run func(context.Context, *Env, A) (string, error), opts ...Option) Tool {
	var o toolOptions
	for _, opt := range opts {
		opt(&o)
	}
	var zero A
	return &typedTool[A]{
		name: name, description: description, run: run, available: o.available,
		schema: schemaOf(reflect.TypeOf(zero)),
	}
}

// NoArgs is the argument struct of a tool that takes none.
type NoArgs struct{}

func schemaOf(t reflect.Type) map[string]any {
	properties := map[string]any{}
	required := []string{}
	if t != nil && t.Kind() == reflect.Struct {
		for i := range t.NumField() {
			f := t.Field(i)
			name, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
			if name == "" || name == "-" {
				continue
			}
			prop := typeSchema(f.Type)
			if desc := f.Tag.Get("desc"); desc != "" {
				prop["description"] = desc
			}
			properties[name] = prop
			if !strings.Contains(opts, "omitempty") {
				required = append(required, name)
			}
		}
	}
	return map[string]any{"type": "object", "properties": properties, "required": required}
}

func typeSchema(t reflect.Type) map[string]any {
	switch t.Kind() {
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int64, reflect.Int32:
		return map[string]any{"type": "integer"}
	case reflect.Float64, reflect.Float32:
		return map[string]any{"type": "number"}
	case reflect.Slice:
		return map[string]any{"type": "array", "items": typeSchema(t.Elem())}
	case reflect.Struct:
		return schemaOf(t)
	}
	return map[string]any{}
}
