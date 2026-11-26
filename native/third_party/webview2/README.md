# WebView2 SDK

`WebView2.h` and `LICENSE.txt` are copied unchanged from Microsoft's
[Microsoft.Web.WebView2](https://www.nuget.org/packages/Microsoft.Web.WebView2)
NuGet package, version **1.0.4370.26** (`build/native/include/WebView2.h`), under
the BSD license in `LICENSE.txt`. `EventToken.h` is lich's: the one declaration
WebView2.h needs from a Windows SDK header mingw-w64 does not ship.

`lich-browser.exe` loads `WebView2Loader.dll` from beside itself at run time.
`native/build-windows.sh` takes it from the same package
(`build/native/x64/WebView2Loader.dll`), checking the package's SHA-256:

    microsoft.web.webview2.1.0.4370.26.nupkg  1c0dacac84b70109b3fd42f90c53a0ba942abc3a87590d7a4e0ccd86e4a06cf4

To move to another version, change both here and in `build-windows.sh`.
