// Copyright 2011 The Go Authors.  All rights reserved.
// Copyright 2013 Manpreet Singh ( junkblocker@yahoo.com ). All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package index

import (
	"log"
	"os"
	"reflect"
	"syscall"
	"unsafe"
)

func mmapFile(f *os.File) mmapData {
	st, err := f.Stat()
	if err != nil {
		log.Fatal(err)
	}
	size := st.Size()
	if int64(int(size+4095)) != size+4095 {
		log.Fatalf("%s: too large for mmap", f.Name())
	}
	if size == 0 {
		return mmapData{f, nil, 0}
	}
	h, err := syscall.CreateFileMapping(syscall.Handle(f.Fd()), nil, syscall.PAGE_READONLY, uint32(size>>32), uint32(size), nil)
	if err != nil {
		log.Fatalf("CreateFileMapping %s: %v", f.Name(), err)
	}

	addr, err := syscall.MapViewOfFile(h, syscall.FILE_MAP_READ, 0, 0, 0)
	if err != nil {
		log.Fatalf("MapViewOfFile %s: %v", f.Name(), err)
	}

	var data []byte

	sh := (*reflect.SliceHeader)(unsafe.Pointer(&data))
	sh.Data = addr
	sh.Len = int(size)
	sh.Cap = int(size)

	return mmapData{f, data, uintptr(h)}
}

func unmmapFile(mm *mmapData) {
	err := syscall.UnmapViewOfFile(uintptr(unsafe.Pointer(&mm.d[0])))
	if err != nil {
		log.Fatal(err)
	}
	err2 := syscall.CloseHandle(syscall.Handle(mm.h))
	if err2 != nil {
		log.Fatal(err2)
	}
	err3 := mm.f.Close()
	if err3 != nil {
		log.Fatal(err3)
	}
}
