/*
Copyright 2025 AmidaWare Inc.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	advapi32           = syscall.NewLazyDLL("advapi32.dll")
	procRegEnumValueW  = advapi32.NewProc("RegEnumValueW")
	procRegDeleteValue = advapi32.NewProc("RegDeleteValueW")
)

func regEnumValue(
	hKey windows.Handle,
	index uint32,
	valueName *uint16,
	valueNameLen *uint32,
	valType *uint32,
	data *byte,
	dataLen *uint32,
) error {
	r1, _, _ := procRegEnumValueW.Call(
		uintptr(hKey),
		uintptr(index),
		uintptr(unsafe.Pointer(valueName)),
		uintptr(unsafe.Pointer(valueNameLen)),
		uintptr(0), // reserved
		uintptr(unsafe.Pointer(valType)),
		uintptr(unsafe.Pointer(data)),
		uintptr(unsafe.Pointer(dataLen)),
	)
	if r1 != 0 {
		return syscall.Errno(r1)
	}
	return nil
}

func regDeleteValue(hKey windows.Handle, valueName *uint16) error {
	r1, _, e1 := syscall.SyscallN(procRegDeleteValue.Addr(),
		uintptr(hKey),
		uintptr(unsafe.Pointer(valueName)),
	)
	if r1 != 0 {
		if e1 != 0 {
			return error(e1)
		}
		return syscall.EINVAL
	}
	return nil
}

// toUint32 tries to convert interface{} to uint32
func toUint32(v interface{}) (uint32, bool) {
	switch val := v.(type) {
	case int:
		return uint32(val), true
	case int32:
		return uint32(val), true
	case int64:
		return uint32(val), true
	case float64: // JSON numbers come as float64
		return uint32(val), true
	case string:
		parsed, err := strconv.ParseUint(val, 10, 32)
		if err == nil {
			return uint32(parsed), true
		}
	}
	return 0, false
}

// toUint64 tries to convert interface{} to uint64
func toUint64(v interface{}) (uint64, bool) {
	switch val := v.(type) {
	case int:
		return uint64(val), true
	case int64:
		return uint64(val), true
	case float64:
		return uint64(val), true
	case string:
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err == nil {
			return parsed, true
		}
	}
	return 0, false
}

// toByteSlice converts interface{} to []byte
// accepts []byte directly OR a hex string like "0A0B0C"
func toByteSlice(v interface{}) ([]byte, bool) {
	switch val := v.(type) {
	case []byte:
		return val, true
	case string:
		// try hex decode
		b, err := hex.DecodeString(strings.ReplaceAll(val, " ", ""))
		if err == nil {
			return b, true
		}
		// fallback: treat as raw string bytes
		return []byte(val), true
	case []interface{}: // JSON array of numbers
		buf := make([]byte, len(val))
		for i, x := range val {
			f, ok := x.(float64)
			if !ok {
				return nil, false
			}
			buf[i] = byte(f)
		}
		return buf, true
	}
	return nil, false
}

func parseUintAuto(s string, bitSize int) (uint64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		return strconv.ParseUint(s[2:], 16, bitSize)
	}
	return strconv.ParseUint(s, 10, bitSize)
}

const (
	HKLM = "HKEY_LOCAL_MACHINE"
	HKCU = "HKEY_CURRENT_USER"
	HKCR = "HKEY_CLASSES_ROOT"
	HKU  = "HKEY_USERS"
	HKCC = "HKEY_CURRENT_CONFIG"

	RegTypeSZ       = "REG_SZ"
	RegTypeExpandSZ = "REG_EXPAND_SZ"
	RegTypeMultiSZ  = "REG_MULTI_SZ"
	RegTypeDWORD    = "REG_DWORD"
	RegTypeQWORD    = "REG_QWORD"
	RegTypeBinary   = "REG_BINARY"
)

// Parse "HKEY_LOCAL_MACHINE\\SOFTWARE" → registry hive + relative path
func getRegistryKeyFromPath(path string) (registry.Key, string, error) {
	parts := strings.SplitN(path, "\\", 2)
	if len(parts) != 2 {
		return 0, "", errors.New("Invalid registry path, Please enter a valid path")
	}

	var hive registry.Key
	switch parts[0] {
	case HKLM:
		hive = registry.LOCAL_MACHINE
	case HKCU:
		hive = registry.CURRENT_USER
	case HKCR:
		hive = registry.CLASSES_ROOT
	case HKU:
		hive = registry.USERS
	case HKCC:
		hive = registry.CURRENT_CONFIG
	default:
		return 0, "", errors.New("unknown registry hive")
	}

	return hive, parts[1], nil
}

type RegistryNode struct {
	Name       string `json:"name"`
	HasSubkeys bool   `json:"hasSubkeys"`
}

type RegistryValue struct {
	Name string      `json:"name"`
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

func listTopLevelHives() []RegistryNode {
	return []RegistryNode{
		{Name: HKCR, HasSubkeys: true},
		{Name: HKCU, HasSubkeys: true},
		{Name: HKLM, HasSubkeys: true},
		{Name: HKU, HasSubkeys: true},
		{Name: HKCC, HasSubkeys: true},
	}
}

func BrowseRegistry(path string, page, pageSize int) ([]RegistryNode, []RegistryValue, bool, error) {
	if strings.ToLower(path) == "computer" || path == "" {
		return listTopLevelHives(), nil, false, nil
	}

	hive, relPath, err := getRegistryKeyFromPath(path)
	if err != nil {
		return nil, nil, false, err
	}

	k, err := registry.OpenKey(hive, relPath, registry.READ)
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to open key %s: %w", path, err)
	}
	defer k.Close()

	subkeys, err := k.ReadSubKeyNames(-1)
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to read subkeys for %s: %w", path, err)
	}

	start := (page - 1) * pageSize
	end := start + pageSize
	if end > len(subkeys) {
		end = len(subkeys)
	}
	hasMore := end < len(subkeys)

	nodes := scanSubkeys(k, subkeys[start:end])

	subPath := syscall.StringToUTF16Ptr(relPath)
	var hKey windows.Handle
	err = windows.RegOpenKeyEx(windows.Handle(hive), subPath, 0, windows.KEY_READ, &hKey)
	if err != nil {
		return nodes, nil, hasMore, fmt.Errorf("RegOpenKeyEx failed: %w", err)
	}
	defer windows.RegCloseKey(hKey)

	values, err := readRegistryValues(hKey)
	if err != nil {
		return nodes, nil, hasMore, err
	}

	return nodes, values, hasMore, nil
}

func scanSubkeys(k registry.Key, names []string) []RegistryNode {
	var result []RegistryNode
	for _, name := range names {
		hasChildren := false
		if subKey, err := registry.OpenKey(k, name, registry.READ); err == nil {
			if subNames, _ := subKey.ReadSubKeyNames(1); len(subNames) > 0 {
				hasChildren = true
			}
			subKey.Close()
		}
		result = append(result, RegistryNode{Name: name, HasSubkeys: hasChildren})
	}
	return result
}

func readRegistryValues(hKey windows.Handle) ([]RegistryValue, error) {
	const (
		initialNameLen = 256
		initialDataLen = 2048

		// Safety caps to avoid runaway allocations / memory abuse.
		maxNameLen = 16 * 1024       // UTF-16 characters
		maxDataLen = 4 * 1024 * 1024 // 4 MB
	)

	var values []RegistryValue
	var index uint32

	for {
		nameLen := uint32(initialNameLen)
		dataLen := uint32(initialDataLen)

		for {
			// Hard caps for safety
			if nameLen == 0 || dataLen == 0 || nameLen > maxNameLen || dataLen > maxDataLen {
				return nil, fmt.Errorf("registry value too large to display (nameLen=%d dataLen=%d)", nameLen, dataLen)
			}

			valueName := make([]uint16, nameLen)
			data := make([]byte, dataLen)

			tmpNameLen := nameLen
			tmpDataLen := dataLen
			var valType uint32

			err := regEnumValue(
				hKey,
				index,
				&valueName[0],
				&tmpNameLen,
				&valType,
				&data[0],
				&tmpDataLen,
			)

			if err == windows.ERROR_NO_MORE_ITEMS {
				return values, nil
			}

			if err == windows.ERROR_MORE_DATA {
				// Grow using required sizes if provided, else exponential backoff.
				if tmpNameLen > nameLen {
					nameLen = tmpNameLen + 16
				} else {
					nameLen *= 2
				}

				if tmpDataLen > dataLen {
					dataLen = tmpDataLen + 256
				} else {
					dataLen *= 2
				}

				// Retry same index
				continue
			}

			if err != nil {
				return nil, fmt.Errorf("RegEnumValue failed: %w", err)
			}

			// Success: decode using tmpNameLen/tmpDataLen
			name := syscall.UTF16ToString(valueName[:tmpNameLen])
			entry := RegistryValue{Name: name}

			switch valType {
			case windows.REG_SZ, windows.REG_EXPAND_SZ:
				// tmpDataLen is bytes; strings are UTF-16
				u16 := (*[1 << 20]uint16)(unsafe.Pointer(&data[0]))[:tmpDataLen/2]
				entry.Type = typeName(valType)
				entry.Data = syscall.UTF16ToString(u16)

			case windows.REG_MULTI_SZ:
				u16 := (*[1 << 20]uint16)(unsafe.Pointer(&data[0]))[:tmpDataLen/2]
				parts := []string{}
				start := 0
				for i, c := range u16 {
					if c == 0 {
						if start < i {
							parts = append(parts, syscall.UTF16ToString(u16[start:i]))
						}
						start = i + 1
					}
				}
				entry.Type = RegTypeMultiSZ
				entry.Data = parts

			case windows.REG_DWORD:
				entry.Type = RegTypeDWORD
				if tmpDataLen < 4 {
					entry.Data = "<invalid DWORD>"
				} else {
					val := binary.LittleEndian.Uint32(data[:4])
					entry.Data = fmt.Sprintf("0x%08X (%d)", val, val)
				}

			case windows.REG_QWORD:
				entry.Type = RegTypeQWORD
				if tmpDataLen < 8 {
					entry.Data = "<invalid QWORD>"
				} else {
					val := binary.LittleEndian.Uint64(data[:8])
					entry.Data = formatQWORD(val)
				}

			case windows.REG_BINARY:
				entry.Type = RegTypeBinary
				// Only use the bytes Windows says are valid
				b := data[:tmpDataLen]
				hexBytes := make([]string, len(b))
				for i := 0; i < len(b); i++ {
					hexBytes[i] = fmt.Sprintf("%02X", b[i])
				}
				entry.Data = strings.Join(hexBytes, " ")

			default:
				entry.Type = fmt.Sprintf("UNKNOWN_%d", valType)
				entry.Data = "<unsupported>"
			}

			values = append(values, entry)
			index++
			break
		}
	}
}

func formatQWORD(val uint64) string {
	if val <= 0xFFFFFFFF {
		return fmt.Sprintf("0x%08X (%d)", val, val)
	}
	return fmt.Sprintf("0x%X (%d)", val, val)
}

func typeName(valType uint32) string {
	switch valType {
	case windows.REG_SZ:
		return RegTypeSZ
	case windows.REG_EXPAND_SZ:
		return RegTypeExpandSZ
	default:
		return fmt.Sprintf("UNKNOWN_%d", valType)
	}
}

func CreateRegistryKey(path string) error {
	hive, relPath, err := getRegistryKeyFromPath(path)
	if err != nil {
		return fmt.Errorf("parsing registry path: %w", err)
	}

	k, exist, err := registry.CreateKey(hive, relPath, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("failed to create registry key '%s': %w", path, err)
	}
	defer k.Close()

	if exist {
		return fmt.Errorf("registry key '%s' already exists", path)
	}

	return nil
}

func DeleteRegistryKey(path string) error {
	cleanPath := strings.TrimRight(path, `\`)
	// Disallow deleting root hives directly
	disallowed := []string{
		HKLM, HKCU, HKCR, HKU, HKCC, "Computer",
	}
	for _, hive := range disallowed {
		if strings.EqualFold(cleanPath, hive) {
			return fmt.Errorf("deleting root hive '%s' is not allowed", hive)
		}
	}

	hive, relPath, err := getRegistryKeyFromPath(cleanPath)
	if err != nil {
		return fmt.Errorf("parsing registry path: %w", err)
	}

	// Open the key with ALL_ACCESS so we can enumerate and delete
	k, err := registry.OpenKey(hive, relPath, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("failed to open key '%s': %w", cleanPath, err)
	}
	defer k.Close()

	// Recursively delete all subkeys first
	subkeys, err := k.ReadSubKeyNames(-1)
	if err != nil {
		return fmt.Errorf("failed to read subkeys for '%s': %w", cleanPath, err)
	}

	for _, sub := range subkeys {
		childPath := cleanPath + `\` + sub
		if err := DeleteRegistryKey(childPath); err != nil {
			return err
		}
	}

	k.Close()

	// Delete the key itself
	if err := registry.DeleteKey(hive, relPath); err != nil {
		return fmt.Errorf("failed to delete registry key '%s': %w", cleanPath, err)
	}

	return nil
}

func RenameRegistryKey(oldPath, newPath string) error {
	oldPath = strings.TrimRight(oldPath, `\`)
	newPath = strings.TrimRight(newPath, `\`)

	oldHive, oldRel, err := getRegistryKeyFromPath(oldPath)
	if err != nil {
		return fmt.Errorf("invalid old path: %w", err)
	}

	newHive, newRel, err := getRegistryKeyFromPath(newPath)
	if err != nil {
		return fmt.Errorf("invalid new path: %w", err)
	}

	oldKey, err := registry.OpenKey(oldHive, oldRel, registry.READ|registry.QUERY_VALUE|registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return fmt.Errorf("failed to open old key path: %w", err)
	}
	defer oldKey.Close()

	newKey, _, err := registry.CreateKey(newHive, newRel, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("failed to create new key: %w", err)
	}
	defer newKey.Close()

	if err := copyRegistryValues(oldKey, newKey); err != nil {
		return err
	}

	if err := copyRegistrySubkeys(oldPath, newPath); err != nil {
		return err
	}

	if err := deleteKeyRecursive(oldHive, oldRel); err != nil {
		return fmt.Errorf("failed to delete old key: %w", err)
	}

	return nil
}

func copyRegistryValues(oldKey, newKey registry.Key) error {
	names, err := oldKey.ReadValueNames(-1)
	if err != nil {
		return fmt.Errorf("failed to read values: %w", err)
	}

	for _, name := range names {
		var copied bool

		// string-like values (REG_SZ / REG_EXPAND_SZ)
		if s, valType, err := oldKey.GetStringValue(name); err == nil {
			switch valType {
			case registry.SZ:
				err = newKey.SetStringValue(name, s)
			case registry.EXPAND_SZ:
				err = newKey.SetExpandStringValue(name, s)
			default:
				// Some hives might report odd types; best-effort fallback
				fmt.Printf("[WARN] GetStringValue returned unexpected type %d for %s, writing as REG_SZ\n", valType, name)
				err = newKey.SetStringValue(name, s)
			}

			if err != nil {
				return fmt.Errorf("failed copying string value %s: %w", name, err)
			}

			copied = true
			continue
		}

		// MULTI_SZ
		if !copied {
			if strs, valType, err := oldKey.GetStringsValue(name); err == nil {
				// valType for MULTI_SZ should be registry.MULTI_SZ
				if valType != registry.MULTI_SZ {
					fmt.Printf("[WARN] GetStringsValue returned unexpected type %d for %s\n", valType, name)
				}

				err = newKey.SetStringsValue(name, strs)
				if err != nil {
					return fmt.Errorf("failed copying multi-string value %s: %w", name, err)
				}

				copied = true
				continue
			}
		}

		// integer values (REG_DWORD / REG_QWORD)
		if !copied {
			if i, valType, err := oldKey.GetIntegerValue(name); err == nil {
				switch valType {
				case registry.DWORD:
					err = newKey.SetDWordValue(name, uint32(i))
				case registry.QWORD:
					err = newKey.SetQWordValue(name, i)
				default:
					fmt.Printf("[WARN] GetIntegerValue returned unexpected type %d for %s, writing as DWORD\n", valType, name)
					err = newKey.SetDWordValue(name, uint32(i))
				}

				if err != nil {
					return fmt.Errorf("failed copying integer value %s: %w", name, err)
				}

				copied = true
				continue
			}
		}

		// binary values (REG_BINARY / REG_NONE-ish stuff)
		if !copied {
			if b, _, err := oldKey.GetBinaryValue(name); err == nil {
				err = newKey.SetBinaryValue(name, b)
				if err != nil {
					return fmt.Errorf("failed copying binary value %s: %w", name, err)
				}

				copied = true
				continue
			}
		}

		if !copied {
			fmt.Printf("[WARN] Could not copy value '%s' (no getter succeeded)\n", name)
		}
	}
	return nil
}

func copyRegistrySubkeys(oldPath, newPath string) error {
	subHive, subRel, err := getRegistryKeyFromPath(oldPath)
	if err != nil {
		return fmt.Errorf("invalid old path: %w", err)
	}

	k, err := registry.OpenKey(subHive, subRel, registry.READ)
	if err != nil {
		return fmt.Errorf("failed to open key: %w", err)
	}
	defer k.Close()

	subkeys, err := k.ReadSubKeyNames(-1)
	if err != nil {
		return fmt.Errorf("failed to read subkeys: %w", err)
	}

	for _, sub := range subkeys {
		subOld := oldPath + `\` + sub
		subNew := newPath + `\` + sub
		if err := RenameRegistryKey(subOld, subNew); err != nil {
			return fmt.Errorf("failed to copy subkey %s: %w", sub, err)
		}
	}
	return nil
}

func deleteKeyRecursive(hive registry.Key, path string) error {
	k, err := registry.OpenKey(hive, path, registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	defer k.Close()

	subkeys, err := k.ReadSubKeyNames(-1)
	if err == nil {
		for _, sub := range subkeys {
			if err := deleteKeyRecursive(hive, path+"\\"+sub); err != nil {
				return err
			}
		}
	}

	return registry.DeleteKey(hive, path)
}

func CreateRegistryValue(path string, name string, valType string, data interface{}) (map[string]interface{}, error) {
	hive, relPath, err := getRegistryKeyFromPath(path)
	if err != nil {
		return nil, fmt.Errorf("parsing registry path: %w", err)
	}

	k, err := registry.OpenKey(hive, relPath, registry.ALL_ACCESS)
	if err != nil {
		return nil, fmt.Errorf("failed to open key for writing '%s': %w", path, err)
	}
	defer k.Close()

	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("registry value name is required")
	}

	// Check if value already exists
	if _, _, err := k.GetValue(name, nil); err == nil {
		return nil, fmt.Errorf("registry value '%s' already exists under '%s'", name, path)
	}

	var displayData interface{}
	t := strings.ToUpper(strings.TrimSpace(valType))

	switch t {
	case RegTypeSZ:
		strVal := ""
		if s, ok := data.(string); ok && s != "" {
			strVal = s
		}
		err = k.SetStringValue(name, strVal)
		displayData = strVal

	case RegTypeExpandSZ:
		strVal := ""
		if s, ok := data.(string); ok && s != "" {
			strVal = s
		}
		err = k.SetExpandStringValue(name, strVal)
		displayData = strVal

	case RegTypeMultiSZ:
		var strs []string
		if data == nil {
			strs = []string{}
		} else {
			switch v := data.(type) {
			case []string:
				strs = v
			case []interface{}:
				for _, item := range v {
					strs = append(strs, fmt.Sprintf("%v", item))
				}
			case string:
				var arr []string
				if uErr := json.Unmarshal([]byte(v), &arr); uErr == nil {
					strs = arr
				} else if strings.Contains(v, ",") {
					strs = strings.Split(v, ",")
				} else {
					strs = []string{v}
				}
			default:
				return nil, fmt.Errorf("expected []string or JSON array for REG_MULTI_SZ, got %T", data)
			}
		}
		err = k.SetStringsValue(name, strs)
		displayData = strs

	case RegTypeDWORD:
		var dword uint32
		if data != nil && data != "" {
			switch v := data.(type) {
			case string:
				parsed, perr := parseUintAuto(v, 32)
				if perr != nil {
					return nil, fmt.Errorf("invalid REG_DWORD value '%s': must be decimal or hex", v)
				}
				dword = uint32(parsed)
			default:
				if num, ok := toUint32(v); ok {
					dword = num
				}
			}
		}
		err = k.SetDWordValue(name, dword)
		displayData = fmt.Sprintf("0x%08X (%d)", dword, dword)

	case RegTypeQWORD:
		var qword uint64
		if data != nil && data != "" {
			switch v := data.(type) {
			case string:
				parsed, perr := parseUintAuto(v, 64)
				if perr != nil {
					return nil, fmt.Errorf("invalid REG_QWORD value '%s': must be decimal or hex", v)
				}
				qword = parsed
			default:
				if num, ok := toUint64(v); ok {
					qword = num
				}
			}
		}
		err = k.SetQWordValue(name, qword)
		displayData = formatQWORD(qword)

	case RegTypeBinary:
		bin := []byte{}
		if data != nil && data != "" {
			if b, ok := toByteSlice(data); ok {
				bin = b
			}
		}
		err = k.SetBinaryValue(name, bin)
		displayData = fmt.Sprintf("% X", bin)

	default:
		return nil, fmt.Errorf("unsupported registry value type: %s", valType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create %s value '%s': %w", valType, name, err)
	}

	result := map[string]interface{}{
		"name": name,
		"type": t,
		"data": displayData,
	}

	return result, nil
}

func DeleteRegistryValue(path string, name string) error {
	path = strings.TrimRight(path, `\`)
	hive, relPath, err := getRegistryKeyFromPath(path)
	if err != nil {
		return fmt.Errorf("parsing registry path: %w", err)
	}

	// open key with write/delete rights
	k, err := registry.OpenKey(hive, relPath, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open key '%s': %w", path, err)
	}
	defer k.Close()

	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("registry value name is required")
	}

	// check if value exists before deleting
	if _, _, err := k.GetValue(name, nil); err != nil {
		return fmt.Errorf("value '%s' does not exist under %s", name, path)
	}

	if err := k.DeleteValue(name); err != nil {
		return fmt.Errorf("failed to delete registry value '%s': %w", name, err)
	}

	return nil
}

func RenameRegistryValue(path, oldName, newName string) (string, error) {
	path = strings.TrimRight(path, `\`)
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)

	if oldName == newName {
		return "", fmt.Errorf("old and new name are the same")
	}

	hive, relPath, err := getRegistryKeyFromPath(path)
	if err != nil {
		return "", fmt.Errorf("parsing registry path: %w", err)
	}

	// Open raw handle
	subPath := syscall.StringToUTF16Ptr(relPath)
	var hKey windows.Handle
	err = windows.RegOpenKeyEx(windows.Handle(hive), subPath, 0, windows.KEY_READ|windows.KEY_WRITE, &hKey)
	if err != nil {
		return "", fmt.Errorf("failed to open key: %w", err)
	}
	defer windows.RegCloseKey(hKey)

	// Read raw value data
	valType, rawData, err := readRegistryValueRaw(hKey, oldName)
	if err != nil {
		return "", fmt.Errorf("failed to read old value: %w", err)
	}

	// Write new value
	if err := writeRegistryValue(hive, relPath, newName, valType, rawData); err != nil {
		return "", fmt.Errorf("failed to write new value: %w", err)
	}

	// Delete old
	if err := regDeleteValue(hKey, syscall.StringToUTF16Ptr(oldName)); err != nil {
		return "", fmt.Errorf("failed to delete old value: %w", err)
	}

	return newName, nil
}

func readRegistryValueRaw(hKey windows.Handle, name string) (uint32, []byte, error) {
	var valType uint32
	var dataLen uint32
	err := windows.RegQueryValueEx(hKey, syscall.StringToUTF16Ptr(name), nil, &valType, nil, &dataLen)
	if err != nil {
		return 0, nil, err
	}

	data := make([]byte, dataLen)
	err = windows.RegQueryValueEx(hKey, syscall.StringToUTF16Ptr(name), nil, &valType, &data[0], &dataLen)
	if err != nil {
		return 0, nil, err
	}

	return valType, data[:dataLen], nil
}

func writeRegistryValue(hive registry.Key, path, name string, valType uint32, data []byte) error {
	k, err := registry.OpenKey(hive, path, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	switch valType {
	case windows.REG_SZ, windows.REG_EXPAND_SZ:
		str := syscall.UTF16ToString((*[1 << 20]uint16)(unsafe.Pointer(&data[0]))[:len(data)/2])
		if valType == windows.REG_SZ {
			return k.SetStringValue(name, str)
		}
		return k.SetExpandStringValue(name, str)

	case windows.REG_MULTI_SZ:
		utf16s := (*[1 << 20]uint16)(unsafe.Pointer(&data[0]))[:len(data)/2]
		parts := []string{}
		start := 0
		for i, c := range utf16s {
			if c == 0 {
				if start < i {
					parts = append(parts, syscall.UTF16ToString(utf16s[start:i]))
				}
				start = i + 1
			}
		}
		return k.SetStringsValue(name, parts)

	case windows.REG_DWORD:
		val := *(*uint32)(unsafe.Pointer(&data[0]))
		return k.SetDWordValue(name, val)

	case windows.REG_QWORD:
		val := *(*uint64)(unsafe.Pointer(&data[0]))
		return k.SetQWordValue(name, val)

	case windows.REG_BINARY:
		return k.SetBinaryValue(name, data)

	default:
		return fmt.Errorf("unsupported registry type: %d", valType)
	}
}

func ModifyRegistryValue(path string, name string, valType string, data interface{}) (map[string]interface{}, error) {
	hive, relPath, err := getRegistryKeyFromPath(path)
	if err != nil {
		return nil, fmt.Errorf("parsing registry path: %w", err)
	}

	k, err := registry.OpenKey(hive, relPath, registry.SET_VALUE)
	if err != nil {
		return nil, fmt.Errorf("failed to open key for writing '%s': %w", path, err)
	}
	defer k.Close()

	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("registry value name is required")
	}

	t := strings.ToUpper(strings.TrimSpace(valType))
	var displayData interface{}

	switch t {

	case RegTypeSZ:
		strVal := ""
		if s, ok := data.(string); ok && s != "" {
			strVal = s
		}
		err = k.SetStringValue(name, strVal)
		displayData = strVal

	case RegTypeExpandSZ:
		strVal := ""
		if s, ok := data.(string); ok && s != "" {
			strVal = s
		}
		err = k.SetExpandStringValue(name, strVal)
		displayData = strVal

	case RegTypeMultiSZ:
		var strs []string
		if data == nil {
			strs = []string{}
		} else {
			switch v := data.(type) {
			case []string:
				strs = v
			case []interface{}:
				for _, item := range v {
					strs = append(strs, fmt.Sprintf("%v", item))
				}
			case string:
				var arr []string
				if uErr := json.Unmarshal([]byte(v), &arr); uErr == nil {
					strs = arr
				} else if strings.Contains(v, ",") {
					strs = strings.Split(v, ",")
				} else {
					strs = []string{v}
				}
			default:
				return nil, fmt.Errorf("expected []string or JSON array for REG_MULTI_SZ, got %T", data)
			}
		}
		err = k.SetStringsValue(name, strs)
		displayData = strs

	case RegTypeDWORD:
		var dword uint32
		if data != nil && data != "" {
			switch v := data.(type) {
			case string:
				parsed, perr := parseUintAuto(v, 32)
				if perr != nil {
					return nil, fmt.Errorf("invalid REG_DWORD value '%s': must be decimal or hex", v)
				}
				dword = uint32(parsed)
			default:
				if num, ok := toUint32(v); ok {
					dword = num
				} else {
					return nil, fmt.Errorf("expected numeric value for REG_DWORD, got %T", v)
				}
			}
		}
		err = k.SetDWordValue(name, dword)
		displayData = fmt.Sprintf("0x%08X (%d)", dword, dword)

	case RegTypeQWORD:
		var qword uint64
		if data != nil && data != "" {
			switch v := data.(type) {
			case string:
				parsed, perr := parseUintAuto(v, 64)
				if perr != nil {
					return nil, fmt.Errorf("invalid REG_QWORD value '%s': must be decimal or hex", v)
				}
				qword = parsed
			default:
				if num, ok := toUint64(v); ok {
					qword = num
				} else {
					return nil, fmt.Errorf("expected numeric value for REG_QWORD, got %T", v)
				}
			}
		}
		err = k.SetQWordValue(name, qword)
		displayData = formatQWORD(qword)

	case RegTypeBinary:
		bin := []byte{}
		if data != nil && data != "" {
			if b, ok := toByteSlice(data); ok {
				bin = b
			}
		}
		err = k.SetBinaryValue(name, bin)
		displayData = fmt.Sprintf("% X", bin)

	default:
		return nil, fmt.Errorf("unsupported registry value type: %s", valType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to modify %s value '%s': %w", valType, name, err)
	}

	result := map[string]interface{}{
		"name": name,
		"type": t,
		"data": displayData,
	}

	return result, nil
}
