package main

import "golang.org/x/sys/windows"

// The "Global\" prefix catches duplicate launches across the entire session, including ones
// started from other user sessions.
const singleInstanceMutexName = `Global\moonkata-sync-server-single-instance`

// acquireSingleInstanceLock returns false if this program is already running. This is the
// standard Windows single-instance pattern based on a named mutex — instead of implementing our
// own file lock, the OS atomically tells us whether another process already holds the mutex.
// No matter how the process dies (normal exit, force-killed, crash), the OS cleans up the mutex
// on its own, so we never need to worry about releasing the lock ourselves.
func acquireSingleInstanceLock() bool {
	name, err := windows.UTF16PtrFromString(singleInstanceMutexName)
	if err != nil {
		return true // Can't even construct the name (practically never happens) — give up on the duplicate check and proceed
	}
	handle, err := windows.CreateMutex(nil, false, name)
	if handle == 0 {
		return true // Mutex creation itself failed — give up on the duplicate check and proceed (safer to not block)
	}
	// CreateMutex returns a valid handle even when it opens a mutex that already exists, while
	// filling err with ERROR_ALREADY_EXISTS (the Win32 GetLastError convention) — so we need to check
	// the handle's validity and this error code separately.
	return err != windows.ERROR_ALREADY_EXISTS
}
