package lua

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

// SecurityConfig defines sandbox limits
type SecurityConfig struct {
	CallStackSize       int  // Maximum call stack depth
	RegistrySize        int  // Maximum registry size
	MemoryLimitMB       int  // Memory limit per state (MB)
	MinimizeStackMemory bool // Enable auto-grow/shrink
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
	L.SetMx(config.MemoryLimitMB)

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
