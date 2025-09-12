package lua

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"
	"github.com/yuin/gopher-lua/parse"
)

// CompiledScript represents shared bytecode (official gopher-lua pattern)
type CompiledScript struct {
	Proto       *lua.FunctionProto // Shared bytecode
	Content     string             // Original content for debugging
	CompileTime time.Time          // When compiled
	Hash        string             // Content hash for cache invalidation
}

// ScriptCompiler manages bytecode compilation and caching
type ScriptCompiler struct {
	mu      sync.RWMutex
	cache   map[string]*CompiledScript
	maxSize int
}

// NewScriptCompiler creates a new script compiler with configurable cache size
func NewScriptCompiler(maxCacheSize int) *ScriptCompiler {
	return &ScriptCompiler{
		cache:   make(map[string]*CompiledScript),
		maxSize: maxCacheSize,
	}
}

// CompileScript compiles Lua script to bytecode using official gopher-lua pattern
func (c *ScriptCompiler) CompileScript(scriptTag, content string) (*CompiledScript, error) {
	// Check cache first
	c.mu.RLock()
	if cached, exists := c.cache[scriptTag]; exists {
		c.mu.RUnlock()
		return cached, nil
	}
	c.mu.RUnlock()

	// Compile to bytecode (official pattern from docs)
	reader := strings.NewReader(content)
	chunk, err := parse.Parse(reader, scriptTag)
	if err != nil {
		return nil, fmt.Errorf("parse error in %s: %w", scriptTag, err)
	}

	proto, err := lua.Compile(chunk, scriptTag)
	if err != nil {
		return nil, fmt.Errorf("compile error in %s: %w", scriptTag, err)
	}

	compiled := &CompiledScript{
		Proto:       proto,
		Content:     content,
		CompileTime: time.Now(),
		Hash:        calculateHash(content),
	}

	// Cache with size limit
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.cache) >= c.maxSize {
		// Simple eviction: remove oldest
		var oldest string
		var oldestTime time.Time
		for key, script := range c.cache {
			if oldest == "" || script.CompileTime.Before(oldestTime) {
				oldest = key
				oldestTime = script.CompileTime
			}
		}
		delete(c.cache, oldest)
	}

	c.cache[scriptTag] = compiled
	return compiled, nil
}

// GetScript retrieves a compiled script from cache
func (c *ScriptCompiler) GetScript(scriptTag string) (*CompiledScript, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	script, exists := c.cache[scriptTag]
	return script, exists
}

// ExecuteWithBytecode executes pre-compiled bytecode (official pattern for 50-70% memory reduction)
func ExecuteWithBytecode(L *lua.LState, script *CompiledScript) error {
	// Use pre-compiled bytecode instead of recompiling
	L.Push(L.NewFunctionFromProto(script.Proto))
	return L.PCall(0, lua.MultRet, nil)
}

// ClearCache removes all cached scripts
func (c *ScriptCompiler) ClearCache() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.cache = make(map[string]*CompiledScript)
}

// CacheSize returns the number of cached scripts
func (c *ScriptCompiler) CacheSize() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return len(c.cache)
}

// calculateHash creates a simple hash for content validation
func calculateHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash[:8]) // Use first 8 bytes for efficiency
}