module goPensionForecast

go 1.25

require (
	github.com/webview/webview_go v0.0.0-20240831120633-6173450d4dd6
	gopkg.in/yaml.v3 v3.0.1
)

require github.com/go-pdf/fpdf v0.9.0

// Use local patched version for webkit2gtk-4.1 support (Ubuntu 24+)
replace github.com/webview/webview_go => ./vendor_patch/webview
