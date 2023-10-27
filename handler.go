package goads

import (
	"errors"
	"fmt"
	"sync"
	"syscall"
	"unsafe"
)

// SymbolHandlers functions as a repository for symbol handlers, functioning akin to a cache table.
// When a symbol is initially read or written, a request is generated to acquire the corresponding handler.
// Subsequent reads or writes will then access this cache table.
var SymbolHandlers map[string]uint32
var SymbolHandlersMutex *sync.RWMutex

// NotificationHandlers functions as a repository for symbol handlers, functioning akin to a cache table.
// When a symbol is initially read or written, a request is generated to acquire the corresponding handler.
// Subsequent reads or writes will then access this cache table.
var NotificationHandlers map[string]uint32
var NotificationHandlersMutex *sync.RWMutex

// SymbolInfoTable functions as a repository for symbol handlers, functioning akin to a cache table.
// When a symbol is initially read or written, a request is generated to acquire the corresponding handler.
// Subsequent reads or writes will then access this cache table.
var SymbolInfoTable map[string]SymbolInfo
var SymbolInfoTableMutex *sync.RWMutex

func (plc *PLC) GeySymInfo(name string) (info SymbolInfo, err error) {
	// Check map is initialized
	if SymbolInfoTable == nil {
		SymbolInfoTable = make(map[string]SymbolInfo)
	}

	// Look up cache map
	// Protected from concurrent read/write
	SymbolHandlersMutex.RLock()
	info, ok := SymbolInfoTable[name]
	SymbolHandlersMutex.RUnlock()

	if !ok {
		// Pointer to the symbol name, this will be transferred to API.
		pointer := unsafe.Pointer(&[]byte(name)[0])

		// Get symbol info
		var bytesRead uint32
		plc.addr.Port = 851
		result, _, _ := syscall.SyscallN(
			plc.rwRequest,
			plc.port,                             // long	port					: Ams port of ADS client
			uintptr(unsafe.Pointer(&(plc.addr))), // AmsAddr*	pServerAddr			: Ams address of ADS server
			uintptr(0xF007),                      // unsigned long indexGroup 		: Index Group
			uintptr(0x0),                         // unsigned long indexOffset 		: Index Offset
			unsafe.Sizeof(info),                  // unsigned long cbReadLength		: count of bytes to read
			uintptr(unsafe.Pointer(&info)),       // void*	pReadData 				: pointer to the client buffer
			uintptr(len(name)),                   // unsigned long cbWriteLength 	: count of bytes to write
			uintptr(pointer),                     // void* pWriteData 				: pointer to the client buffer
			uintptr(unsafe.Pointer(&bytesRead)),  // unsigned long* pcbReturn 		: count of bytes read
		)

		if result != 0 {
			return info, errors.New("did not fetch symbol info")
		} else {
			// Store new info
			// Protected from concurrent read/write
			SymbolInfoTableMutex.Lock()
			SymbolInfoTable[name] = info
			SymbolInfoTableMutex.Unlock()
			return info, nil
		}
	} else {
		return info, nil
	}

}

func (plc *PLC) GetSymbolHandler(name string) (handler uint32, err error) {
	// Check SymbolHandlers is initialized
	if SymbolHandlers == nil {
		SymbolHandlers = make(map[string]uint32)
	}

	// Look up SymbolHandlers for given name
	// Protected from concurrent read/write
	SymbolHandlersMutex.RLock()
	handler, ok := SymbolHandlers[name]
	SymbolHandlersMutex.RUnlock()

	if ok {
		// This handler is in cache table however in plc runtime the handler may change,
		// so it is not secure to use cache table. An extra validation may be required.
		return handler, nil
	} else {
		// Pointer to the symbol name, this will be transferred to API.
		pointer := unsafe.Pointer(&[]byte(name)[0])

		// Get symbol handler
		var bytesRead uint32
		plc.addr.Port = 851
		result, _, _ := syscall.SyscallN(
			plc.rwRequest,
			plc.port,                             // long	port					: Ams port of ADS client
			uintptr(unsafe.Pointer(&(plc.addr))), // AmsAddr*	pServerAddr			: Ams address of ADS server
			uintptr(0xF003),                      // unsigned long indexGroup 		: Index Group
			uintptr(0x0),                         // unsigned long indexOffset 		: Index Offset
			unsafe.Sizeof(handler),               // unsigned long cbReadLength		: count of bytes to read
			uintptr(unsafe.Pointer(&handler)),    // void*	pReadData 				: pointer to the client buffer
			uintptr(len(name)),                   // unsigned long cbWriteLength 	: count of bytes to write
			uintptr(pointer),                     // void* pWriteData 				: pointer to the client buffer
			uintptr(unsafe.Pointer(&bytesRead)),  // unsigned long* pcbReturn 		: count of bytes read
		)

		if result != 0 {
			return 0, errors.New(fmt.Sprintf("GetSymbolHandler [%s]: did not obtain symbol handle, ads return code: %d", name, result))
		} else {
			// Protected from concurrent read/write
			SymbolHandlersMutex.Lock()
			SymbolHandlers[name] = handler
			SymbolHandlersMutex.Unlock()
			return handler, nil
		}
	}
}

// ReleaseAllSymbolHandler clears all registered symbol handlers.
func (plc *PLC) ReleaseAllSymbolHandler(name string) error {
	var err error
	for key, _ := range SymbolHandlers {
		if localError := plc.ReleaseSymbolHandler(key); localError != nil {
			err = localError
		}
	}
	return err
}

// ReleaseSymbolHandler makes API call to release handler.
func (plc *PLC) ReleaseSymbolHandler(name string) (err error) {
	// Look up cache map
	// Protected from concurrent read/write
	SymbolHandlersMutex.RLock()
	handler, ok := SymbolHandlers[name]
	SymbolHandlersMutex.RUnlock()

	if ok {
		// Call ads api to release handle
		plc.addr.Port = 851
		result, _, _ := syscall.SyscallN(
			plc.wRequest,
			plc.port,                           // Ams port of ADS client
			uintptr(unsafe.Pointer(&plc.addr)), // Ams address of ADS server
			uintptr(0xF006),                    // index group in ADS server interface
			uintptr(handler),                   // index offset in ADS server interface
			uintptr(4),                         // count of bytes to write
			uintptr(unsafe.Pointer(&handler)),  // pointer to the client buffer
		)

		// Delete from cache table
		// Protected from concurrent read/write
		SymbolHandlersMutex.Lock()
		delete(SymbolHandlers, name)
		SymbolHandlersMutex.Unlock()

		if result != 0 {
			return errors.New(fmt.Sprintf("did not release the symbol handle, ads return code: %d", result))
		} else {
			return nil
		}
	} else {
		return nil
	}
}

// ClearAllDeviceNotification clears all registered notifications.
func (plc *PLC) ClearAllDeviceNotification() error {
	var err error
	for key, _ := range NotificationHandlers {
		if localError := plc.ClearDeviceNotification(key); localError != nil {
			err = localError
		}
	}
	return err
}

// ClearDeviceNotification implements the API Call AdsSyncDelDeviceNotificationReqEx.
// Deletes the registered device notifications from PLC. The device notification handlers are stored as a slice.
// This routine ranges over the slice, and makes API request to delete corresponding device notification.
// The unterminated notifications may cause in excessive source usage, it is recommended to delete device notifications
// afterwards.
func (plc *PLC) ClearDeviceNotification(name string) (err error) {
	// Look up cache map
	NotificationHandlersMutex.RLock()
	handler, ok := NotificationHandlers[name]
	NotificationHandlersMutex.RUnlock()

	if ok {
		plc.addr.Port = 851
		result, _, _ := syscall.SyscallN(
			plc.delDeviceNotification,
			plc.port,                           // long	port					: Ams port of ADS client
			uintptr(unsafe.Pointer(&plc.addr)), // AmsAddr*	pServerAddr			: Ams address of ADS server
			uintptr(handler),                   // unsigned long hNotification	: Notification handle
		)

		if result != 0 {
			return errors.New(fmt.Sprintf("did not clear notification, ads return code: %d", result))
		} else {
			// Delete from cache table
			// Protected from concurrent read/write
			SymbolHandlersMutex.Lock()
			delete(SymbolHandlers, name)
			SymbolHandlersMutex.Unlock()
			return nil
		}
	} else {
		return nil
	}
}
