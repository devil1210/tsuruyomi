package extensions

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dop251/goja"
)

// GlobalEngine is the global shared JavaScript extension execution engine.
var GlobalEngine = NewEngine()

// Engine manages the JavaScript execution environments for running Tachiyomi extensions.
type Engine struct {
	mu         sync.RWMutex
	vmPool     sync.Pool
	registry   map[string]*ExtensionSource
	httpBridge *HTTPBridge
}

// ExtensionSource holds the compiled program/script metadata.
type ExtensionSource struct {
	ID      string
	PkgName string
	Script  *goja.Program
}

// NewEngine creates an Extension JS Engine wrapper.
func NewEngine() *Engine {
	e := &Engine{
		registry:   make(map[string]*ExtensionSource),
		httpBridge: NewHTTPBridge(),
	}
	e.vmPool.New = func() interface{} {
		vm := goja.New()
		e.injectBuiltins(vm)
		return vm
	}
	return e
}

// injectBuiltins populates the Goja runtime environment with global Android/Mihon emulated polyfills and bridges.
func (e *Engine) injectBuiltins(vm *goja.Runtime) {
	// 1. console.log bridge
	_ = vm.Set("console", map[string]interface{}{
		"log": func(args ...interface{}) {
			fmt.Print("JS LOG: ")
			for _, arg := range args {
				fmt.Printf("%v ", arg)
			}
			fmt.Println()
		},
	})

	// 2. HTTP network request bridge
	_ = vm.Set("httpReq", func(method, url string, body string, headers map[string]string) (string, error) {
		return e.httpBridge.Request(method, url, body, headers)
	})

	// Inject a friendly javascript fetch wrapper
	_, _ = vm.RunString(`
		function fetch(url, options) {
			options = options || {};
			var method = options.method || 'GET';
			var body = options.body || '';
			var headers = options.headers || {};
			try {
				var res = httpReq(method, url, body, headers);
				return {
					text: function() { return res; },
					json: function() { return JSON.parse(res); }
				};
			} catch (e) {
				throw new Error("fetch failed: " + e.message);
			}
		}
	`)

	// 3. HTML DOM parser bridge
	_ = vm.Set("parseHTML", func(html string) (*DOMNode, error) {
		return ParseHTML(html)
	})

	// Tachiyomi extension environment builtins stub
	_ = vm.Set("Tachiyomi", map[string]interface{}{
		"version": "1.0",
	})
}

// LoadExtension compiles and caches an extension JS file.
func (e *Engine) LoadExtension(filePath string, pkgName string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading extension script: %w", err)
	}

	program, err := goja.Compile(pkgName, string(content), false)
	if err != nil {
		return fmt.Errorf("compiling script: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	e.registry[pkgName] = &ExtensionSource{
		ID:      pkgName,
		PkgName: pkgName,
		Script:  program,
	}

	return nil
}

// LoadAllExtensions loads all .js extension files inside the given directory path.
func (e *Engine) LoadAllExtensions(dir string) error {
	// Create directory if not exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		_ = os.MkdirAll(dir, 0755)
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading extensions directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".js") {
			pkgName := strings.TrimSuffix(entry.Name(), ".js")
			filePath := filepath.Join(dir, entry.Name())
			if err := e.LoadExtension(filePath, pkgName); err != nil {
				fmt.Printf("Warning: Failed to load extension %s: %v\n", entry.Name(), err)
			} else {
				fmt.Printf("Successfully loaded extension JS: %s\n", entry.Name())
			}
		}
	}

	return nil
}

// RunExtensionMethod executes a method from a loaded extension source using a leased VM.
func (e *Engine) RunExtensionMethod(pkgName string, methodName string, args ...interface{}) (goja.Value, error) {
	e.mu.RLock()
	source, ok := e.registry[pkgName]
	e.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("extension '%s' not loaded", pkgName)
	}

	// Lease VM
	vm := e.vmPool.Get().(*goja.Runtime)
	defer e.vmPool.Put(vm)

	// Execute base script to load functions into runtime context
	_, err := vm.RunProgram(source.Script)
	if err != nil {
		return nil, fmt.Errorf("executing script bundle: %w", err)
	}

	// Get target method
	fnVal := vm.Get(methodName)
	if fnVal == nil {
		return nil, fmt.Errorf("method '%s' not found in extension '%s'", methodName, pkgName)
	}

	fn, ok := goja.AssertFunction(fnVal)
	if !ok {
		return nil, fmt.Errorf("value of '%s' is not a function", methodName)
	}

	// Convert Go args to Goja values
	jsArgs := make([]goja.Value, len(args))
	for i, arg := range args {
		jsArgs[i] = vm.ToValue(arg)
	}

	// Call function
	res, err := fn(goja.Undefined(), jsArgs...)
	if err != nil {
		return nil, fmt.Errorf("executing method '%s': %w", methodName, err)
	}

	return res, nil
}
