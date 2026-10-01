package main

/*
#cgo LDFLAGS: -framework CoreAudio -framework CoreFoundation
#include <CoreAudio/CoreAudio.h>
#include <stdlib.h>

static AudioObjectID findDevice(const char *want) {
	AudioObjectPropertyAddress a = {kAudioHardwarePropertyDevices, kAudioObjectPropertyScopeGlobal, kAudioObjectPropertyElementMain};
	UInt32 size = 0;
	if (AudioObjectGetPropertyDataSize(kAudioObjectSystemObject, &a, 0, NULL, &size) != noErr) return 0;
	AudioObjectID *ids = malloc(size);
	AudioObjectGetPropertyData(kAudioObjectSystemObject, &a, 0, NULL, &size, ids);
	CFStringRef wantRef = CFStringCreateWithCString(NULL, want, kCFStringEncodingUTF8);
	AudioObjectID found = 0;
	for (UInt32 i = 0; i < size / sizeof(AudioObjectID) && !found; i++) {
		CFStringRef name = NULL;
		UInt32 sz = sizeof(name);
		AudioObjectPropertyAddress na = {kAudioObjectPropertyName, kAudioObjectPropertyScopeGlobal, kAudioObjectPropertyElementMain};
		if (AudioObjectGetPropertyData(ids[i], &na, 0, NULL, &sz, &name) == noErr && name) {
			if (CFStringCompare(name, wantRef, kCFCompareCaseInsensitive) == kCFCompareEqualTo) found = ids[i];
			CFRelease(name);
		}
	}
	CFRelease(wantRef);
	free(ids);
	return found;
}

static int inputChannelCount(AudioObjectID dev) {
	AudioObjectPropertyAddress a = {kAudioDevicePropertyStreamConfiguration, kAudioDevicePropertyScopeInput, kAudioObjectPropertyElementMain};
	UInt32 size = 0;
	if (AudioObjectGetPropertyDataSize(dev, &a, 0, NULL, &size) != noErr) return 0;
	AudioBufferList *list = malloc(size);
	int n = 0;
	if (AudioObjectGetPropertyData(dev, &a, 0, NULL, &size, list) == noErr)
		for (UInt32 i = 0; i < list->mNumberBuffers; i++) n += list->mBuffers[i].mNumberChannels;
	free(list);
	return n;
}

static int inputChannelName(AudioObjectID dev, int ch, char *buf, int len) {
	CFStringRef name = NULL;
	UInt32 sz = sizeof(name);
	AudioObjectPropertyAddress a = {kAudioObjectPropertyElementName, kAudioDevicePropertyScopeInput, (UInt32)ch};
	if (AudioObjectGetPropertyData(dev, &a, 0, NULL, &sz, &name) != noErr || !name) return 0;
	Boolean ok = CFStringGetCString(name, buf, len, kCFStringEncodingUTF8);
	CFRelease(name);
	return ok;
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// hostInNames returns the input channel names CoreAudio (and therefore Logic) sees for a device.
func hostInNames(device string) ([]string, error) {
	cname := C.CString(device)
	defer C.free(unsafe.Pointer(cname))
	dev := C.findDevice(cname)
	if dev == 0 {
		return nil, fmt.Errorf("CoreAudio device %q not found", device)
	}
	n := int(C.inputChannelCount(dev))
	names := make([]string, n)
	buf := (*C.char)(C.malloc(256))
	defer C.free(unsafe.Pointer(buf))
	for ch := 1; ch <= n; ch++ {
		if C.inputChannelName(dev, C.int(ch), buf, 256) != 0 {
			names[ch-1] = C.GoString(buf)
		}
	}
	return names, nil
}
