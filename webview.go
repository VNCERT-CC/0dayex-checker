package main

import (
	"sync/atomic"
	"time"

	"github.com/jchv/go-webview2"
)

func webviewDispatch(w webview2.WebView, f func()) {
	var fired uint32
	dpFunc := func() {
		if !atomic.CompareAndSwapUint32(&fired, 0, 1) {
			return
		}
		f()
	}
	w.Dispatch(dpFunc)
	for atomic.LoadUint32(&fired) == 0 {
		w.Dispatch(dpFunc)
		time.Sleep(100 * time.Millisecond)
	}
}
