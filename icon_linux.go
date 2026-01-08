//go:build linux && !console

package main

/*
#cgo pkg-config: gtk+-3.0
#include <gtk/gtk.h>
#include <stdlib.h>

int set_window_icon_from_data(GtkWindow *window, const unsigned char *data, int len) {
    if (!data || len <= 0) {
        return 0;
    }

    GError *error = NULL;
    GdkPixbufLoader *loader = gdk_pixbuf_loader_new();
    if (!gdk_pixbuf_loader_write(loader, data, len, &error)) {
        if (error) g_error_free(error);
        g_object_unref(loader);
        return 0;
    }
    if (!gdk_pixbuf_loader_close(loader, &error)) {
        if (error) g_error_free(error);
        g_object_unref(loader);
        return 0;
    }

    GdkPixbuf *pixbuf = gdk_pixbuf_loader_get_pixbuf(loader);
    if (!pixbuf) {
        g_object_unref(loader);
        return 0;
    }

    // Set as default icon for all windows (helps with taskbar)
    GList *icon_list = NULL;
    icon_list = g_list_append(icon_list, pixbuf);
    gtk_window_set_default_icon_list(icon_list);
    g_list_free(icon_list);

    // Also set on specific window if provided
    if (window) {
        gtk_window_set_icon(window, pixbuf);
    }

    g_object_unref(loader);
    return 1;
}

void set_app_id(const char *app_id) {
    g_set_prgname(app_id);
    g_set_application_name("Pension Forecast");
}
*/
import "C"

import (
	_ "embed"
	"unsafe"
)

//go:embed assets/icon.png
var iconPNG []byte

var iconInitialized = false

// SetWindowIcon sets the window icon for GTK windows on Linux
func SetWindowIcon(windowPtr unsafe.Pointer) {
	if iconInitialized || len(iconPNG) == 0 {
		return
	}
	iconInitialized = true

	// Set application ID for better desktop integration (enables taskbar icon)
	appID := C.CString("pension-forecast")
	C.set_app_id(appID)
	C.free(unsafe.Pointer(appID))

	var window *C.GtkWindow
	if windowPtr != nil {
		window = (*C.GtkWindow)(windowPtr)
	}
	C.set_window_icon_from_data(window, (*C.uchar)(unsafe.Pointer(&iconPNG[0])), C.int(len(iconPNG)))
}
