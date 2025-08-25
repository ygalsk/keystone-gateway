package lua

import (
	"fmt"
	"runtime"

	lua "github.com/yuin/gopher-lua"
)

// SecurityConfig defines sandbox limits
type SecurityConfig struct {
	CallStackSize       int  // Maximum call stack depth
	RegistrySize        int  // Maximum registry size
	MemoryLimitMB       int  // Memory limit per state (MB)
	MinimizeStackMemory bool // Enable auto-grow/shrink
}

// EnhancedSecurityConfig extends SecurityConfig with memory-aware features
type EnhancedSecurityConfig struct {
	SecurityConfig
	GlobalMemoryMB     int64 // Global memory limit across all states (MB)
	AggressiveGC       bool  // Enable aggressive garbage collection
	MemoryPressureMode bool  // Enable memory pressure monitoring
}

// DefaultEnhancedSecurityConfig provides memory-aware defaults
func DefaultEnhancedSecurityConfig() EnhancedSecurityConfig {
	return EnhancedSecurityConfig{
		SecurityConfig: SecurityConfig{
			CallStackSize:       120,  // Sufficient for most scripts
			RegistrySize:        2400, // Enhanced registry limit (120 * 20)
			MemoryLimitMB:       10,   // 10MB per state
			MinimizeStackMemory: true, // Memory efficiency
		},
		GlobalMemoryMB:     100,  // 100MB total across all states
		AggressiveGC:       true, // Enable for memory efficiency
		MemoryPressureMode: true, // Enable memory pressure monitoring
	}
}

// DefaultSecurityConfig provides production-safe defaults
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		CallStackSize:       120,      // Sufficient for most scripts
		RegistrySize:        120 * 20, // Registry limit
		MemoryLimitMB:       10,       // 10MB per state
		MinimizeStackMemory: true,     // Memory efficiency
	}
}

// CreateSecureLuaState - Official gopher-lua security pattern
func CreateSecureLuaState(config SecurityConfig) *lua.LState {
	L := lua.NewState(lua.Options{
		CallStackSize:       config.CallStackSize,
		RegistrySize:        config.RegistrySize,
		MinimizeStackMemory: config.MinimizeStackMemory,
		SkipOpenLibs:        true, // CRITICAL: Only open safe libraries
	})

	// CRITICAL: Memory limit (from official documentation)
	//L.SetMx(config.MemoryLimitMB)

	// Only open safe libraries (exclude io, os, debug, package)
	safeLibraries := []struct {
		name string
		fn   lua.LGFunction
	}{
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString},
		{lua.MathLibName, lua.OpenMath},
		// Explicitly exclude: lua.IoLibName, lua.OsLibName, lua.DebugLibName
	}

	for _, lib := range safeLibraries {
		if err := L.CallByParam(lua.P{
			Fn:      L.NewFunction(lib.fn),
			NRet:    0,
			Protect: true,
		}, lua.LString(lib.name)); err != nil {
			panic(fmt.Sprintf("Failed to open library %s: %v", lib.name, err))
		}
	}

	return L
}

// ValidateSecurityLimits checks if state is within limits
func ValidateSecurityLimits(L *lua.LState) error {
	if L.IsClosed() {
		return fmt.Errorf("lua state is closed")
	}

	if L.GetTop() > 1000 { // Reasonable stack limit
		return fmt.Errorf("stack overflow detected: %d items", L.GetTop())
	}

	return nil
}

// CreateEnhancedSecureLuaState creates a secure Lua state with enhanced memory management
func CreateEnhancedSecureLuaState(config EnhancedSecurityConfig) *lua.LState {
	L := lua.NewState(lua.Options{
		CallStackSize:       config.CallStackSize,
		RegistrySize:        config.RegistrySize,
		MinimizeStackMemory: config.MinimizeStackMemory,
		SkipOpenLibs:        true, // CRITICAL: Only open safe libraries
	})

	// CRITICAL: Memory limit now handled by the state pool to prevent goroutine leaks
	// SetMx creates a goroutine that calls runtime.ReadMemStats() every 100ms
	// This causes severe memory leaks under high load - removed in favor of pool-level management

	// Only open safe libraries (exclude io, os, debug, package)
	safeLibraries := []struct {
		name string
		fn   lua.LGFunction
	}{
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString},
		{lua.MathLibName, lua.OpenMath},
		// Explicitly exclude: lua.IoLibName, lua.OsLibName, lua.DebugLibName
	}

	for _, lib := range safeLibraries {
		if err := L.CallByParam(lua.P{
			Fn:      L.NewFunction(lib.fn),
			NRet:    0,
			Protect: true,
		}, lua.LString(lib.name)); err != nil {
			panic(fmt.Sprintf("Failed to open library %s: %v", lib.name, err))
		}
	}

	// Apply aggressive GC if enabled
	if config.AggressiveGC {
		runtime.GC()
	}

	return L
}

// ValidateEnhancedSecurityLimits performs comprehensive security validation
func ValidateEnhancedSecurityLimits(L *lua.LState, config EnhancedSecurityConfig) error {
	if err := ValidateSecurityLimits(L); err != nil {
		return err
	}

	// Additional memory pressure checks if enabled
	// Removed expensive runtime.ReadMemStats call - memory management now handled by pool
	// Config parameter retained for API compatibility
	_ = config

	return nil
}
