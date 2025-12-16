package lua

import lua "github.com/aarzilli/golua/lua"

// RestorePCall creates pcall and xpcall as aliases to unsafe_pcall and unsafe_xpcall.
// This is needed because golua renames these functions for safety, but LuaRocks modules expect them.
// Safe for LuaRocks since they don't call back to Go.
func RestorePCall(L *lua.State) {
	// Create pcall as alias to unsafe_pcall
	L.GetGlobal("unsafe_pcall")
	if !L.IsNil(-1) {
		L.SetGlobal("pcall")
	} else {
		L.Pop(1)
	}

	// Create xpcall as alias to unsafe_xpcall
	L.GetGlobal("unsafe_xpcall")
	if !L.IsNil(-1) {
		L.SetGlobal("xpcall")
	} else {
		L.Pop(1)
	}
}
