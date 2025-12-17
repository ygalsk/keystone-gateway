// +build luajit

package lua

/*
#cgo pkg-config: luajit
#include <stdlib.h>
#include <lua.h>
#include <lauxlib.h>
#include <lualib.h>
#include <string.h>

// Batch set string fields into a Lua table at stack index `table_idx`
// This reduces N*3 CGO calls (PushString key + PushString value + RawSet) to 1 CGO call
//
// fields: array of key-value pairs as [key1, value1, key2, value2, ...]
// count: number of key-value pairs (length of fields array / 2)
static void batch_set_string_fields(lua_State *L, int table_idx, const char** fields, int count) {
    // Convert relative index to absolute for safety
    if (table_idx < 0 && table_idx > LUA_REGISTRYINDEX) {
        table_idx = lua_gettop(L) + table_idx + 1;
    }

    // Set each key-value pair using lua_rawset (bypasses metamethods)
    for (int i = 0; i < count; i++) {
        const char* key = fields[i * 2];
        const char* value = fields[i * 2 + 1];

        lua_pushstring(L, key);    // Push key
        lua_pushstring(L, value);  // Push value
        lua_rawset(L, table_idx);  // table[key] = value, pops key and value
    }
}

// Batch set string key-value pairs into a nested table
// This creates a new table, populates it, and sets it as a field in the parent table
//
// parent_idx: stack index of parent table
// field_name: name of the field in parent table
// pairs: array of [key1, value1, key2, value2, ...]
// count: number of pairs
static void batch_set_table_field(lua_State *L, int parent_idx, const char* field_name, const char** pairs, int count) {
    // Convert relative index to absolute
    if (parent_idx < 0 && parent_idx > LUA_REGISTRYINDEX) {
        parent_idx = lua_gettop(L) + parent_idx + 1;
    }

    // Push field name
    lua_pushstring(L, field_name);

    // Create new table with pre-allocated hash size
    lua_createtable(L, 0, count);
    int table_idx = lua_gettop(L);

    // Populate the table
    for (int i = 0; i < count; i++) {
        const char* key = pairs[i * 2];
        const char* value = pairs[i * 2 + 1];

        lua_pushstring(L, key);
        lua_pushstring(L, value);
        lua_rawset(L, table_idx);
    }

    // Set parent[field_name] = new_table
    lua_rawset(L, parent_idx);  // Pops field_name and new_table
}
*/
import "C"
import (
	"reflect"
	"unsafe"

	"github.com/aarzilli/golua/lua"
)

// getLuaState extracts the internal *C.lua_State from golua's State struct
// The State struct has an unexported field 's' that holds the C pointer
func getLuaState(L *lua.State) *C.lua_State {
	// Use reflection to access the unexported 's' field
	v := reflect.ValueOf(L).Elem()
	field := v.FieldByName("s")
	// Use unsafe to get the pointer value
	return *(**C.lua_State)(unsafe.Pointer(field.UnsafeAddr()))
}

// BatchSetStringFields sets multiple string fields in a Lua table with a single CGO call
// This reduces 3N CGO calls (PushString + PushString + RawSet) × N to 1 CGO call
//
// Example:
//   BatchSetStringFields(L, -1, map[string]string{
//       "method": "GET",
//       "path": "/api/users",
//   })
func BatchSetStringFields(L *lua.State, tableIdx int, fields map[string]string) {
	if len(fields) == 0 {
		return
	}

	// Convert Go map to C array of strings [key1, value1, key2, value2, ...]
	cFields := make([]*C.char, 0, len(fields)*2)
	defer func() {
		// Free all C strings
		for _, cstr := range cFields {
			C.free(unsafe.Pointer(cstr))
		}
	}()

	for key, value := range fields {
		cFields = append(cFields, C.CString(key))
		cFields = append(cFields, C.CString(value))
	}

	// Call C function with array of string pointers
	C.batch_set_string_fields(
		getLuaState(L),
		C.int(tableIdx),
		(**C.char)(unsafe.Pointer(&cFields[0])),
		C.int(len(fields)),
	)
}

// BatchSetTableField creates a nested table, populates it, and sets it as a field
// This reduces the CGO overhead for setting tables like headers, params, query
//
// Example:
//   BatchSetTableField(L, -1, "headers", map[string]string{
//       "Content-Type": "application/json",
//       "Accept": "text/html",
//   })
func BatchSetTableField(L *lua.State, parentIdx int, fieldName string, pairs map[string]string) {
	if len(pairs) == 0 {
		// Still need to create an empty table
		L.PushString(fieldName)
		L.NewTable()
		L.RawSet(parentIdx)
		return
	}

	// Convert fieldName to C string
	cFieldName := C.CString(fieldName)
	defer C.free(unsafe.Pointer(cFieldName))

	// Convert pairs to C array
	cPairs := make([]*C.char, 0, len(pairs)*2)
	defer func() {
		for _, cstr := range cPairs {
			C.free(unsafe.Pointer(cstr))
		}
	}()

	for key, value := range pairs {
		cPairs = append(cPairs, C.CString(key))
		cPairs = append(cPairs, C.CString(value))
	}

	// Call C function
	C.batch_set_table_field(
		getLuaState(L),
		C.int(parentIdx),
		cFieldName,
		(**C.char)(unsafe.Pointer(&cPairs[0])),
		C.int(len(pairs)),
	)
}
