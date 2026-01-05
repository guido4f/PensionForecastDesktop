Pension Forecast Simulator
==========================

This package contains the Pension Forecast Simulator for your platform.

QUICK START
-----------
Run the included script for your platform:
  - macOS/Linux: ./run.sh
  - Windows: run.bat (double-click)

BINARIES INCLUDED
-----------------
  *-web      Web server mode (recommended) - Opens in your browser
  *-ui       Desktop UI - Embedded browser window (native look)
  *-console  Console mode - Command line interface

MANUAL USAGE
------------
You can run binaries directly with these flags:
  ./goPensionForecast-*-web -web         Start web server
  ./goPensionForecast-*-ui -ui           Start desktop UI
  ./goPensionForecast-*-console          Start console mode

For more options:
  ./goPensionForecast-*-web -help

CONFIGURATION
-------------
The application uses config.yaml in the current directory.
If not found, it creates one with default values.

DOCUMENTATION
-------------
Full glossary available at: /glossary (when running web server)
Source code: https://github.com/guido4f/go-PensionForecast

PLATFORM NOTES
--------------
macOS: The run.sh script automatically removes quarantine attributes.
       If you get "unidentified developer" errors, run: xattr -d com.apple.quarantine <binary>

Linux: UI mode requires GTK3 and WebKit2GTK:
       sudo apt-get install libgtk-3-0 libwebkit2gtk-4.1-0

Windows: If Windows Defender blocks the app, click "More info" then "Run anyway".
