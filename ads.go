package goads

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"sync"
	"syscall"
	"unsafe"
)

type NotificationAttribute struct {
	Length    uint32
	TransMode TransMode
	MaxDelay  uint32
	CycleTime uint32
}

type NotificationHeader struct {
	TimeStamp    int64
	Notification uint32
	SampleSize   uint32
	Data         [1]byte
}

type CallBack func(addr *AmsAddr, pNotification *NotificationHeader, handle uint32) uintptr

type SymbolInfo struct {
	IndexGroup  uint32
	IndexOffset uint32
	Length      uint32
}

type Symbol struct {
	Name               string   `json:"name" db:"name"`
	Type               string   `json:"-" db:"type"`
	CallBack           CallBack `json:"-"`
	Size               uint32   `json:"-"`
	handler            uint32
	notificationHandle uint32
	buffer             [64]byte
}

type AmsAddr struct {
	NetId [6]byte
	Port  uint16
}

type PLC struct {
	addr                  AmsAddr
	apiHandler            syscall.Handle
	port                  uintptr
	adsState              uint16
	deviceState           uint16
	notifications         []uint32
	rwRequest             uintptr
	wRequest              uintptr
	rRequest              uintptr
	delDeviceNotification uintptr
	notificationReq       uintptr
	closePort             uintptr
	readState             uintptr
	writeState            uintptr
}

func NewPLC() *PLC {
	h, err := syscall.LoadLibrary("TcAdsDll.dll")
	if err != nil {
		log.Fatal("can not load TcAdsDll")
	}

	plc := new(PLC)
	plc.apiHandler = h

	plc.rwRequest, _ = syscall.GetProcAddress(plc.apiHandler, "AdsSyncReadWriteReqEx2")
	plc.wRequest, _ = syscall.GetProcAddress(plc.apiHandler, "AdsSyncWriteReqEx")
	plc.rRequest, _ = syscall.GetProcAddress(plc.apiHandler, "AdsSyncReadReqEx2")
	plc.notificationReq, _ = syscall.GetProcAddress(plc.apiHandler, "AdsSyncAddDeviceNotificationReqEx")
	plc.delDeviceNotification, _ = syscall.GetProcAddress(plc.apiHandler, "AdsSyncDelDeviceNotificationReqEx")
	plc.closePort, _ = syscall.GetProcAddress(plc.apiHandler, "AdsPortCloseEx")
	plc.readState, _ = syscall.GetProcAddress(plc.apiHandler, "AdsSyncReadStateReqEx")
	plc.writeState, _ = syscall.GetProcAddress(plc.apiHandler, "AdsSyncWriteControlReqEx")

	// Open new ads port
	if err := plc.OpenPort(); err != nil {
		log.Fatalln(err.Error())
	}

	// Get local AMS address
	if err := plc.GetLocalAddress(); err != nil {
		log.Fatalln(err.Error())
	}

	// Read/write protection for symbol handle table
	SymbolHandlersMutex = new(sync.RWMutex)

	// Read/write protection for notification handle table
	NotificationHandlersMutex = new(sync.RWMutex)

	// Read/write protection for symbol info table
	SymbolInfoTableMutex = new(sync.RWMutex)

	/*// Restart PLC
	if err := plc.WriteState(StateReset, 0); err != nil {
		log.Fatalln(err.Error())
	}

	// Periodic task that monitor the ADS state for every 30 seconds.
	// If ADS server is not in running mode after 120 seconds,
	// the timeout occurs. The timeout is monitored by a context with timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Periodic function
	go func() {
		done := make(chan bool, 1)
		for {
			select {
			case <-done:
				cancel()
				break
			case <-time.After(8 * time.Second):
				plc.ReadState()
				if plc.adsState == StateRun {
					done <- true
				}
			}
		}
	}()

	select {
	case <-ctx.Done():
		switch ctx.Err() {
		case context.DeadlineExceeded:
			log.Fatalln("timeout is occurred: plc is not in run mode")
		case context.Canceled:
			// ok
		}
	}*/

	return plc
}

// ReadState reads the ADS and Device states.
// In order to read states, PortSystemService must be used.
func (plc *PLC) ReadState() {
	plc.addr.Port = PortSystemService

	result, _, _ := syscall.SyscallN(
		plc.readState,
		plc.port,                                  // Ams port of ADS client
		uintptr(unsafe.Pointer(&plc.addr)),        // Ams address of ADS server
		uintptr(unsafe.Pointer(&plc.adsState)),    // pointer to client buffer
		uintptr(unsafe.Pointer(&plc.deviceState)), // pointer to the client buffer
	)

	if result != 0 {
		log.Fatalf("did not read state of the ADS server: %d", result)
		return
	}
}

// WriteState writes the desired ADS state.
// In order to write states, PortSystemService must be used.
func (plc *PLC) WriteState(adsState, deviceState uint16) error {
	plc.addr.Port = PortSystemService

	result, _, _ := syscall.SyscallN(
		plc.writeState,
		plc.port,                           // Ams port of ADS client
		uintptr(unsafe.Pointer(&plc.addr)), // Ams address of ADS server
		uintptr(adsState),                  // pointer to client buffer
		uintptr(deviceState),               // pointer to the client buffer
		uintptr(0),                         // pointer to the client buffer
		uintptr(0),                         // pointer to the client buffer
	)
	if result != 0 {
		return errors.New(fmt.Sprintf("did not change the status of plc, ads return code: %d", result))
	}
	return nil
}

// ClosePort implements the API Call AdsPortCloseEx.
// It closes the active port in usage. It is recommended to close port after exiting the main program.
func (plc *PLC) ClosePort() {
	result, _, _ := syscall.SyscallN(plc.closePort, plc.port)
	if result != 0 {
		log.Fatalf("did not close the PLC port, ads return code: %d", result)
		return
	}
}

// OpenPort implements the API Call AdsPortOpenEx.
// It opens a new port to communicate with PLC.
func (plc *PLC) OpenPort() error {
	openPortProc, _ := syscall.GetProcAddress(plc.apiHandler, "AdsPortOpenEx")

	plc.port, _, _ = syscall.SyscallN(openPortProc)
	if plc.port == 0 {
		return errors.New("did not open the port, ads return code")
	}
	return nil
}

// GetLocalAddress implements the API Call AdsGetLocalAddressEx.
// It gets the local AMS address.
func (plc *PLC) GetLocalAddress() error {
	getLocalAddressProc, _ := syscall.GetProcAddress(plc.apiHandler, "AdsGetLocalAddressEx")

	result, _, _ := syscall.SyscallN(
		getLocalAddressProc,
		plc.port,
		uintptr(unsafe.Pointer(&plc.addr)),
	)

	if result != 0 {
		return errors.New(fmt.Sprintf("did not obtain local AMS address, ads return code: %d", result))
	}
	return nil
}

// ReadByName reads the value of a plc symbol. It first checks the SymbolHandlers to access symbol handler.
// Then make read request to ADS. The returned bytes then converted teh value of given PlcType.
// An example usage is given below:
// x := new(ads.PlcTypeBool)
// err := e.PLC.ReadByName("SystemActions.REQUEST_SYSTEM_STOP.ActionLog.Stored", x)
//
//	if err != nil {
//		fmt.Println(err)
//	}
//
// fmt.Println(x.Value)
func (plc *PLC) ReadByName(sym string, val interface{}) (err error) {
	// Get symbol handler
	handler, err := plc.GetSymbolHandler(sym)
	if err != nil {
		return err
	}

	// Pointer to data
	var pointer uintptr

	// get pointer
	if reflect.ValueOf(val).Kind() == reflect.Ptr || reflect.ValueOf(val).Kind() == reflect.Slice {
		pointer = reflect.ValueOf(val).Pointer()
	} else {
		fmt.Println(sym)
		return errors.New("expected pointer but got value")
	}

	// decide size of the val
	size := uint32(DataSize(val))

	// Make read request
	var readBytes uint32
	plc.addr.Port = 851
	result, _, _ := syscall.SyscallN(
		plc.rRequest,
		plc.port,                           // Ams port of ADS client
		uintptr(unsafe.Pointer(&plc.addr)), // Ams address of ADS server
		uintptr(0xF005),                    // index group in ADS server interface
		uintptr(handler),                   // index offset in ADS server interface
		uintptr(size),                      // count of bytes to read
		uintptr(pointer),                   // index offset in ADS server interface
		uintptr(readBytes),                 // pointer to the client buffer
	)

	// Render result
	if result != 0 {
		// ADS returned with error, if symbol handler is not valid remove it from the cache table
		if result == 1809 {
			fmt.Println("symbol handler is not valid trying, removing from cache table")
			err := plc.ReleaseSymbolHandler(sym)
			if err != nil {
				return errors.New(fmt.Sprintf("ReadByName [%s]: did not release the handler, ads return code %d", sym, result))
			}
		}

		return errors.New(fmt.Sprintf("ReadByName [%s]: did not read from the plc, ads return code %d", sym, result))
	}

	return nil
}

// WriteByName writes the desired value to a plc symbol. It first checks the SymbolHandlers to access symbol handler.
// Then make write request to ADS. An example usage is given below:
// y := new(bool)
// *y = true
// err = e.PLC.WriteByName("SystemActions.REQUEST_SYSTEM_STOP.Signal", y)
//
//	if err != nil {
//		fmt.Println(err)
//	}
func (plc *PLC) WriteByName(sym string, val interface{}) (err error) {

	// Pointer to data
	var pointer uintptr

	// decide size of the val
	size := uint32(DataSize(val))

	// get pointer
	if reflect.ValueOf(val).Kind() == reflect.Ptr || reflect.ValueOf(val).Kind() == reflect.Slice {
		pointer = reflect.ValueOf(val).Pointer()
	} else {
		fmt.Println(sym)
		return errors.New("expected pointer but got value")
	}

	// Get symbol handler
	handler, err := plc.GetSymbolHandler(sym)
	if err != nil {
		return err
	}

	// Write to plc
	plc.addr.Port = 851
	result, _, _ := syscall.SyscallN(
		plc.wRequest,
		plc.port,                           // Ams port of ADS client
		uintptr(unsafe.Pointer(&plc.addr)), // Ams address of ADS server
		uintptr(0xF005),                    // index group in ADS server interface
		uintptr(handler),                   // index offset in ADS server interface
		uintptr(size),                      // count of bytes to write
		uintptr(pointer),                   // pointer to the client buffer
	)

	// Render result
	if result != 0 {
		// ADS returned with error, if symbol handler is not valid remove it from the cache table
		if result == 1809 {
			fmt.Println("symbol handler is not valid trying, removing from cache table")
			err := plc.ReleaseSymbolHandler(sym)
			if err != nil {
				return errors.New(fmt.Sprintf("ReadByName [%s]: did not release the handler, ads return code %d", sym, result))
			}
		}
		return errors.New(fmt.Sprintf("WriteByName [%s]: did not write to plc, ads return code %d", sym, result))
	}

	return nil

}

// NotificationHandle implements the API Call AdsSyncAddDeviceNotificationReqEx.
// First, the handler is obtained if the symbol is not read before. The obtained handler
// will be stored in memory for future reading and writing operations. After the handler is obtained,
// notification callback function is attached. The PLC, triggers this callback functions when according to the
// selected TransMode. An example usage is given below:
/*
z := new(ads.PlcTypeInt)
f := func(variable ads.PlcType) func(*ads.AmsAddr, *ads.NotificationHeader, uint32) uintptr {
	return func(addr *ads.AmsAddr, pNotification *ads.NotificationHeader, handle uint32) uintptr {
		// readBytes := C.GoBytes(unsafe.Pointer(&(pNotification.Data)), C.int(pNotification.SampleSize))
		readBytes := make([]byte, pNotification.SampleSize)
		copy(readBytes, pNotification.Data[:])

		err := variable.FromBuffer(readBytes)
		if err != nil {
			fmt.Println("notification error")
			return 0
		}
		return 0
	}
}

callback := f(z)

err = e.PLC.NotificationHandle("SystemActions.REQUEST_SYSTEM_STOP.ActionLog.Level", callback)
if err != nil {
	fmt.Println(err)
}
*/
func (plc *PLC) NotificationHandle(sym string, callBack CallBack) (err error) {
	// Get symbol handler
	handler, err := plc.GetSymbolHandler(sym)
	if err != nil {
		return err
	}

	// Get symbol information
	info, err := plc.GeySymInfo(sym)
	if err != nil {
		return err
	}

	// Check SymbolHandlers is initialized
	if NotificationHandlers == nil {
		NotificationHandlers = make(map[string]uint32)
	}

	// Look up SymbolHandlers for given name
	notificationHandle, ok := NotificationHandlers[sym]

	if !ok {
		// Pointer to callback function
		ptr := syscall.NewCallback(callBack)

		adsNotificationAttrib := &NotificationAttribute{
			Length:    info.Length,
			TransMode: TransServerOnCha,
			MaxDelay:  0,
			CycleTime: 500000,
		}

		plc.addr.Port = 851
		result, _, _ := syscall.SyscallN(
			plc.notificationReq,
			plc.port,                           // long	port					: Ams port of ADS client
			uintptr(unsafe.Pointer(&plc.addr)), // AmsAddr*	pServerAddr			: Ams address of ADS server
			uintptr(0xF005),                    // unsigned long indexGroup 	: Index Group
			uintptr(handler),                   // unsigned long indexOffset 	: Index Offset
			uintptr(unsafe.Pointer(adsNotificationAttrib)), // AdsNotificationAttrib* pNoteAttrib: attributes of notification request
			ptr,              // PAdsNotificationFuncEx pNoteFunc:address of notification callback
			uintptr(handler), // unsigned long hUser:  user handle
			uintptr(unsafe.Pointer(&notificationHandle)), // unsigned long *pNotification:pointer to notification handle (return value)
		)

		if result != 0 {
			return errors.New(fmt.Sprintf("NotificationHandle [%s]: did not write to plc, ads return code %d", sym, result))
		} else {
			NotificationHandlersMutex.Lock()
			NotificationHandlers[sym] = notificationHandle
			NotificationHandlersMutex.Unlock()
		}
	} else {
		return errors.New(fmt.Sprintf("There is already a NotificationHandle for %s", sym))
	}

	return nil
}

// DataSize returns the size of the data required to represent the data when encoded.
// It returns zero if the type cannot be implemented by the fast path in Read or Write.
func DataSize(data any) int {
	switch data := data.(type) {
	case bool, int8, uint8, *bool, *int8, *uint8:
		return 1
	case []bool:
		return len(data)
	case []int8:
		return len(data)
	case []uint8:
		return len(data)
	case int16, uint16, *int16, *uint16:
		return 2
	case []int16:
		return 2 * len(data)
	case []uint16:
		return 2 * len(data)
	case int32, uint32, *int32, *uint32:
		return 4
	case []int32:
		return 4 * len(data)
	case []uint32:
		return 4 * len(data)
	case int64, uint64, *int64, *uint64:
		return 8
	case []int64:
		return 8 * len(data)
	case []uint64:
		return 8 * len(data)
	case float32, *float32:
		return 4
	case float64, *float64:
		return 8
	case []float32:
		return 4 * len(data)
	case []float64:
		return 8 * len(data)
	}
	return 0
}
