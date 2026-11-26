// The WebView2 engine: each tab is a WebView2 controller in a window of
// lich-browser's (docs/lich-browser.md). Everything runs on the thread that
// called lich_browser_init, the one with the message loop.
#include <windows.h>
#include <objidl.h>
#include <shlwapi.h>

#include <stdio.h>

#include <functional>
#include <string>
#include <vector>

#include "WebView2.h"
#include "browser_windows.h"

extern "C" {
void lichBrowserRun(uintptr_t handle);
void lichDrawn(uintptr_t key);
uintptr_t lichDevtoolsKey(uintptr_t inspected);
void lichDevtoolsOpened(uintptr_t key, LichTab *tab);
void lichDevtoolsClosed(uintptr_t inspected);
void lichLoad(uintptr_t key, int step, char *uri);
void lichLoadFailed(uintptr_t key, char *uri, char *message, int download);
void lichNavigationFailed(uintptr_t key, char *uri, int status);
void lichTabChanged(uintptr_t key, int icon);
uintptr_t lichTabCreate(uintptr_t opener);
void lichTabCreated(uintptr_t key, LichTab *tab);
void lichTabClosing(uintptr_t key);
void lichCrashed(uintptr_t key);
int lichDialog(uintptr_t key, LichDialog *dialog, int type, char *message, char *text);
char *lichDownloadPath(uintptr_t page, char *suggested);
void lichDownload(uintptr_t page, int guid, char *url, char *name, char *path, int state, int64_t received,
                  int64_t total);
void lichClosed(uintptr_t page);
void lichCDPDone(uintptr_t handle, int ok, char *json);
void lichCDPEvent(uintptr_t tab, char *name, char *json);
void lichMessage(uintptr_t page, char *json);
}

// A tab no window shows sits here, off every screen yet shown, so WebView2
// runs it as a page in sight (occlusion is off: lich_browser_init).
static const int OFF_SCREEN = -20000;
static const UINT RUN = WM_APP + 1;
static const UINT_PTR DEVTOOLS_TIMER = 1;
static const UINT_PTR ABORTED_TIMER = 2;
// A navigation that turns into a download completes first, aborted, and its
// download starts after (measured on GitHub's windows-latest, WebView2 of
// 2026-11): the abort is held this long for the download to claim it.
static const UINT ABORTED_HOLD_MS = 2000;
// The inspector's window is looked for this often, this many times, after
// OpenDevToolsWindow, which answers before the window exists.
static const UINT DEVTOOLS_POLL_MS = 100;
static const int DEVTOOLS_POLLS = 100;

// Handler is a WebView2 callback interface I made of a function: its Invoke's
// arguments are read off I's own.
template <class I, class M> struct Handler;
template <class I, class... A> struct Handler<I, HRESULT (STDMETHODCALLTYPE I::*)(A...)> : I {
	std::function<HRESULT(A...)> f;
	ULONG refs = 1;
	explicit Handler(std::function<HRESULT(A...)> f) : f(std::move(f)) {}
	HRESULT STDMETHODCALLTYPE QueryInterface(REFIID, void **out) override {
		*out = this;
		AddRef();
		return S_OK;
	}
	ULONG STDMETHODCALLTYPE AddRef() override { return ++refs; }
	ULONG STDMETHODCALLTYPE Release() override {
		if (--refs == 0) {
			delete this;
			return 0;
		}
		return refs;
	}
	HRESULT STDMETHODCALLTYPE Invoke(A... a) override { return f(a...); }
};

// on is f as an I, holding one reference for its caller to release once it
// handed it over.
template <class I, class F> static I *on(F f) { return new Handler<I, decltype(&I::Invoke)>(f); }

static std::wstring wide(const char *s) {
	if (!s) {
		return L"";
	}
	int n = MultiByteToWideChar(CP_UTF8, 0, s, -1, NULL, 0);
	std::wstring w(n > 0 ? n - 1 : 0, L'\0');
	if (n > 1) {
		MultiByteToWideChar(CP_UTF8, 0, s, -1, &w[0], n);
	}
	return w;
}

static std::string narrow(const wchar_t *w) {
	if (!w) {
		return "";
	}
	int n = WideCharToMultiByte(CP_UTF8, 0, w, -1, NULL, 0, NULL, NULL);
	std::string s(n > 0 ? n - 1 : 0, '\0');
	if (n > 1) {
		WideCharToMultiByte(CP_UTF8, 0, w, -1, &s[0], n, NULL, NULL);
	}
	return s;
}

// taken is a string WebView2 handed over, freed.
static std::string taken(LPWSTR w) {
	std::string s = narrow(w);
	CoTaskMemFree(w);
	return s;
}

struct LichPage {
	uintptr_t key;
	std::wstring script;
	std::wstring profile;
	// popped is the page's own window while it is popped out, showing shown.
	HWND popped;
	LichTab *shown;
};

struct LichTab {
	LichPage *page;
	uintptr_t key;
	HWND host;
	ICoreWebView2Controller *controller;
	ICoreWebView2 *view;
	// ready is whether the view is made; until then later holds what was
	// asked of it, in order.
	bool ready;
	std::vector<std::function<void()>> later;
	double scale;
	std::string uri, title;
	std::vector<unsigned char> icon;
	bool loading;
	// pending is the main-frame navigation not committed yet, the one a
	// download may turn out to be; downloaded: it did.
	std::string pending;
	bool downloaded;
	// aborted holds an aborted navigation's failure for its download.
	bool aborted;
	// inspector: this tab holds inspected's DevTools window, devtools.
	bool inspector;
	LichTab *inspected;
	HWND devtools;
	int polls;
	bool closed;
};

struct LichDialog {
	ICoreWebView2ScriptDialogOpeningEventArgs *args;
	ICoreWebView2Deferral *deferral;
};

static HWND runner;
static ICoreWebView2Environment *env;
static HWND app;
static const wchar_t *TAB_CLASS = L"LichBrowserTab";
static const wchar_t *POPPED_CLASS = L"LichBrowserWindow";

// fit sizes what a tab's window holds to the window.
static void fit(LichTab *tab, HWND window) {
	RECT r;
	GetClientRect(window, &r);
	if (tab->inspector) {
		if (tab->devtools) {
			MoveWindow(tab->devtools, 0, 0, r.right, r.bottom, TRUE);
		}
		return;
	}
	if (tab->controller) {
		tab->controller->put_Bounds(r);
	}
}

static void find_devtools(LichTab *d);
static void abort_told(LichTab *tab);

static LRESULT CALLBACK tab_proc(HWND w, UINT msg, WPARAM wp, LPARAM lp) {
	LichTab *tab = (LichTab *)GetWindowLongPtrW(w, GWLP_USERDATA);
	if (tab && msg == WM_SIZE) {
		fit(tab, w);
	}
	if (tab && msg == WM_TIMER && wp == DEVTOOLS_TIMER) {
		find_devtools(tab);
		return 0;
	}
	if (tab && msg == WM_TIMER && wp == ABORTED_TIMER) {
		abort_told(tab);
		return 0;
	}
	return DefWindowProcW(w, msg, wp, lp);
}

static LRESULT CALLBACK popped_proc(HWND w, UINT msg, WPARAM wp, LPARAM lp) {
	LichPage *page = (LichPage *)GetWindowLongPtrW(w, GWLP_USERDATA);
	if (page && msg == WM_SIZE && page->shown) {
		fit(page->shown, w);
	}
	// The person closing the window ends the page: the wire tells the
	// backend, and Go closes the page, this window with it.
	if (page && msg == WM_CLOSE) {
		lichClosed(page->key);
		return 0;
	}
	return DefWindowProcW(w, msg, wp, lp);
}

static LRESULT CALLBACK run_proc(HWND w, UINT msg, WPARAM wp, LPARAM lp) {
	if (msg == RUN) {
		lichBrowserRun((uintptr_t)lp);
		return 0;
	}
	return DefWindowProcW(w, msg, wp, lp);
}

void lich_browser_invoke(uintptr_t handle) { PostMessageW(runner, RUN, 0, (LPARAM)handle); }

static char *failure(const char *what, HRESULT hr) {
	char *s = (char *)malloc(256);
	snprintf(s, 256, "%s (HRESULT 0x%08lx)", what, (unsigned long)hr);
	return s;
}

typedef HRESULT(STDAPICALLTYPE *CreateEnvironment)(PCWSTR, PCWSTR, ICoreWebView2EnvironmentOptions *,
                                                     ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandler *);

// loader is WebView2Loader.dll from beside this executable, where the
// installer puts it.
static HMODULE loader(void) {
	wchar_t path[MAX_PATH];
	DWORD n = GetModuleFileNameW(NULL, path, MAX_PATH);
	if (n == 0 || n == MAX_PATH) {
		return NULL;
	}
	PathRemoveFileSpecW(path);
	std::wstring dll = std::wstring(path) + L"\\WebView2Loader.dll";
	return LoadLibraryW(dll.c_str());
}

char *lich_browser_init(const char *folder) {
	SetProcessDpiAwarenessContext(DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2);
	HRESULT hr = CoInitializeEx(NULL, COINIT_APARTMENTTHREADED);
	if (FAILED(hr)) {
		return failure("COM", hr);
	}
	HINSTANCE self = GetModuleHandleW(NULL);
	WNDCLASSW c = {};
	c.hInstance = self;
	c.lpfnWndProc = run_proc;
	c.lpszClassName = L"LichBrowserRun";
	RegisterClassW(&c);
	c.lpfnWndProc = tab_proc;
	c.lpszClassName = TAB_CLASS;
	c.hCursor = LoadCursor(NULL, IDC_ARROW);
	RegisterClassW(&c);
	c.lpfnWndProc = popped_proc;
	c.lpszClassName = POPPED_CLASS;
	RegisterClassW(&c);
	runner = CreateWindowExW(0, L"LichBrowserRun", L"", 0, 0, 0, 0, 0, HWND_MESSAGE, NULL, self, NULL);

	// Chromium stops drawing and throttles a window it finds covered or off
	// screen, which every tab no window shows is: the page has to run as
	// one in sight, as it does on Linux.
	SetEnvironmentVariableW(L"WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS",
	                        L"--disable-features=CalculateNativeWinOcclusion --disable-backgrounding-occluded-windows "
	                        L"--disable-renderer-backgrounding --disable-background-timer-throttling");
	HMODULE dll = loader();
	if (!dll) {
		return failure("no WebView2Loader.dll beside lich-browser.exe", HRESULT_FROM_WIN32(GetLastError()));
	}
	CreateEnvironment create = (CreateEnvironment)GetProcAddress(dll, "CreateCoreWebView2EnvironmentWithOptions");
	if (!create) {
		return failure("WebView2Loader.dll has no CreateCoreWebView2EnvironmentWithOptions", E_NOINTERFACE);
	}
	HRESULT made = E_PENDING;
	auto done = on<ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandler>(
	    [&](HRESULT result, ICoreWebView2Environment *e) -> HRESULT {
		    made = result;
		    if (SUCCEEDED(result) && e) {
			    env = e;
			    env->AddRef();
		    }
		    return S_OK;
	    });
	std::wstring data = wide(folder);
	hr = create(NULL, data.c_str(), NULL, done);
	done->Release();
	if (FAILED(hr)) {
		return failure("WebView2 is not installed or would not start", hr);
	}
	MSG m;
	while (made == E_PENDING && GetMessageW(&m, NULL, 0, 0) > 0) {
		TranslateMessage(&m);
		DispatchMessageW(&m);
	}
	if (FAILED(made) || !env) {
		return failure("WebView2 made no environment", made);
	}
	return NULL;
}

void lich_browser_main(void) {
	MSG m;
	while (GetMessageW(&m, NULL, 0, 0) > 0) {
		TranslateMessage(&m);
		DispatchMessageW(&m);
	}
}

void lich_browser_quit(void) { PostQuitMessage(0); }

void lich_browser_set_app(uint64_t hwnd) { app = (HWND)(uintptr_t)hwnd; }

void lich_free(void *p) { free(p); }

LichPage *lich_page_new(uintptr_t key, const char *script) {
	LichPage *page = new LichPage();
	page->key = key;
	page->script = wide(script);
	// Each page's InPrivate profile of its own: its cookies and storage are
	// its tabs' alone and end with it.
	// Every lich-browser shares WebView2's data folder, so the name carries
	// the process too.
	page->profile = L"p" + std::to_wstring(GetCurrentProcessId()) + L"-" + std::to_wstring(key);
	return page;
}

void lich_page_free(LichPage *page) { delete page; }

int lich_page_popped(LichPage *page) { return page->popped != NULL; }

static HWND new_host(LichTab *tab, int width, int height) {
	HWND w = CreateWindowExW(WS_EX_TOOLWINDOW | WS_EX_NOACTIVATE, TAB_CLASS, L"", WS_POPUP | WS_CLIPCHILDREN, OFF_SCREEN,
	                         OFF_SCREEN, width, height, NULL, NULL, GetModuleHandleW(NULL), NULL);
	SetWindowLongPtrW(w, GWLP_USERDATA, (LONG_PTR)tab);
	ShowWindow(w, SW_SHOWNOACTIVATE);
	return w;
}

// when runs f now if the tab's view is made, else once it is.
static void when(LichTab *tab, std::function<void()> f) {
	if (tab->closed) {
		return;
	}
	if (tab->ready) {
		f();
		return;
	}
	tab->later.push_back(std::move(f));
}

static void cdp(LichTab *tab, const wchar_t *method, const wchar_t *params, std::function<void(HRESULT, LPCWSTR)> done) {
	auto h = on<ICoreWebView2CallDevToolsProtocolMethodCompletedHandler>(
	    [done](HRESULT hr, LPCWSTR json) -> HRESULT {
		    if (done) {
			    done(hr, json);
		    }
		    return S_OK;
	    });
	tab->view->CallDevToolsProtocolMethod(method, params, h);
	h->Release();
}

// The DevTools protocol events Go reads (cdp_windows.go).
static const wchar_t *cdp_events[] = {L"Network.requestWillBeSent", L"Network.loadingFinished",
                                      L"Network.loadingFailed", L"Page.fileChooserOpened"};

static void listen_cdp(LichTab *tab) {
	for (const wchar_t *name : cdp_events) {
		ICoreWebView2DevToolsProtocolEventReceiver *r = NULL;
		if (FAILED(tab->view->GetDevToolsProtocolEventReceiver(name, &r)) || !r) {
			continue;
		}
		std::string event = narrow(name);
		uintptr_t key = tab->key;
		auto h = on<ICoreWebView2DevToolsProtocolEventReceivedEventHandler>(
		    [event, key](ICoreWebView2 *, ICoreWebView2DevToolsProtocolEventReceivedEventArgs *args) -> HRESULT {
			    LPWSTR json = NULL;
			    args->get_ParameterObjectAsJson(&json);
			    std::string s = taken(json);
			    lichCDPEvent(key, (char *)event.c_str(), (char *)s.c_str());
			    return S_OK;
		    });
		EventRegistrationToken t;
		r->add_DevToolsProtocolEventReceived(h, &t);
		h->Release();
		r->Release();
	}
}

static void read_stream(IStream *s, std::vector<unsigned char> &out) {
	out.clear();
	LARGE_INTEGER zero = {};
	s->Seek(zero, STREAM_SEEK_SET, NULL);
	unsigned char buf[16384];
	ULONG n = 0;
	while (SUCCEEDED(s->Read(buf, sizeof buf, &n)) && n > 0) {
		out.insert(out.end(), buf, buf + n);
	}
}

static void changed(LichTab *tab) { lichTabChanged(tab->key, 0); }

static void refresh_uri(LichTab *tab) {
	LPWSTR s = NULL;
	if (SUCCEEDED(tab->view->get_Source(&s))) {
		tab->uri = taken(s);
	}
}

// abort_told tells the held abort as the failure it is: no download claimed
// it.
static void abort_told(LichTab *tab) {
	KillTimer(tab->host, ABORTED_TIMER);
	tab->aborted = false;
	lichNavigationFailed(tab->key, (char *)tab->pending.c_str(), (int)COREWEBVIEW2_WEB_ERROR_STATUS_CONNECTION_ABORTED);
}

static void connect_loads(LichTab *tab) {
	ICoreWebView2 *v = tab->view;
	EventRegistrationToken t;
	auto starting = on<ICoreWebView2NavigationStartingEventHandler>(
	    [tab](ICoreWebView2 *, ICoreWebView2NavigationStartingEventArgs *a) -> HRESULT {
		    LPWSTR u = NULL;
		    a->get_Uri(&u);
		    std::string uri = taken(u);
		    tab->loading = true;
		    if (tab->aborted) {
			    abort_told(tab);
		    }
		    tab->pending = uri;
		    tab->downloaded = false;
		    lichLoad(tab->key, 0, (char *)uri.c_str());
		    changed(tab);
		    return S_OK;
	    });
	v->add_NavigationStarting(starting, &t);
	starting->Release();
	auto content = on<ICoreWebView2ContentLoadingEventHandler>(
	    [tab](ICoreWebView2 *, ICoreWebView2ContentLoadingEventArgs *) -> HRESULT {
		    tab->pending.clear();
		    refresh_uri(tab);
		    lichLoad(tab->key, 2, (char *)tab->uri.c_str());
		    return S_OK;
	    });
	v->add_ContentLoading(content, &t);
	content->Release();
	auto completed = on<ICoreWebView2NavigationCompletedEventHandler>(
	    [tab](ICoreWebView2 *, ICoreWebView2NavigationCompletedEventArgs *a) -> HRESULT {
		    BOOL ok = FALSE;
		    a->get_IsSuccess(&ok);
		    COREWEBVIEW2_WEB_ERROR_STATUS status = COREWEBVIEW2_WEB_ERROR_STATUS_UNKNOWN;
		    a->get_WebErrorStatus(&status);
		    refresh_uri(tab);
		    tab->loading = false;
		    if (tab->downloaded) {
			    // Told as a download already (connect_downloads).
		    } else if (!ok && status == COREWEBVIEW2_WEB_ERROR_STATUS_CONNECTION_ABORTED && !tab->pending.empty()) {
			    tab->aborted = true;
			    SetTimer(tab->host, ABORTED_TIMER, ABORTED_HOLD_MS, NULL);
		    } else if (ok) {
			    lichLoad(tab->key, 3, (char *)tab->uri.c_str());
		    } else {
			    lichNavigationFailed(tab->key, (char *)tab->uri.c_str(), (int)status);
		    }
		    changed(tab);
		    return S_OK;
	    });
	v->add_NavigationCompleted(completed, &t);
	completed->Release();
	auto source = on<ICoreWebView2SourceChangedEventHandler>(
	    [tab](ICoreWebView2 *, ICoreWebView2SourceChangedEventArgs *) -> HRESULT {
		    refresh_uri(tab);
		    changed(tab);
		    return S_OK;
	    });
	v->add_SourceChanged(source, &t);
	source->Release();
	auto title = on<ICoreWebView2DocumentTitleChangedEventHandler>([tab](ICoreWebView2 *v, IUnknown *) -> HRESULT {
		LPWSTR s = NULL;
		if (SUCCEEDED(v->get_DocumentTitle(&s))) {
			tab->title = taken(s);
		}
		changed(tab);
		return S_OK;
	});
	v->add_DocumentTitleChanged(title, &t);
	title->Release();
}

static void connect_icon(LichTab *tab) {
	ICoreWebView2_15 *v15 = NULL;
	if (FAILED(tab->view->QueryInterface(IID_ICoreWebView2_15, (void **)&v15)) || !v15) {
		return;
	}
	EventRegistrationToken t;
	auto h = on<ICoreWebView2FaviconChangedEventHandler>([tab, v15](ICoreWebView2 *, IUnknown *) -> HRESULT {
		auto got = on<ICoreWebView2GetFaviconCompletedHandler>([tab](HRESULT hr, IStream *s) -> HRESULT {
			if (tab->closed) {
				return S_OK;
			}
			if (SUCCEEDED(hr) && s) {
				read_stream(s, tab->icon);
			} else {
				tab->icon.clear();
			}
			lichTabChanged(tab->key, 1);
			return S_OK;
		});
		v15->GetFavicon(COREWEBVIEW2_FAVICON_IMAGE_FORMAT_PNG, got);
		got->Release();
		return S_OK;
	});
	v15->add_FaviconChanged(h, &t);
	h->Release();
	// The tab's view holds the handler, the handler v15.
}

static void connect_dialogs(LichTab *tab) {
	EventRegistrationToken t;
	auto h = on<ICoreWebView2ScriptDialogOpeningEventHandler>(
	    [tab](ICoreWebView2 *, ICoreWebView2ScriptDialogOpeningEventArgs *a) -> HRESULT {
		    LichDialog *d = new LichDialog();
		    d->args = a;
		    a->AddRef();
		    a->GetDeferral(&d->deferral);
		    COREWEBVIEW2_SCRIPT_DIALOG_KIND kind = COREWEBVIEW2_SCRIPT_DIALOG_KIND_ALERT;
		    a->get_Kind(&kind);
		    LPWSTR m = NULL, text = NULL;
		    a->get_Message(&m);
		    a->get_DefaultText(&text);
		    std::string message = taken(m), def = taken(text);
		    lichDialog(tab->key, d, (int)kind, (char *)message.c_str(), (char *)def.c_str());
		    return S_OK;
	    });
	tab->view->add_ScriptDialogOpening(h, &t);
	h->Release();
}

void lich_dialog_answer(LichDialog *d, int accept, const char *text) {
	if (accept) {
		if (text) {
			std::wstring w = wide(text);
			d->args->put_ResultText(w.c_str());
		}
		d->args->Accept();
	}
	d->deferral->Complete();
	d->deferral->Release();
	d->args->Release();
	delete d;
}

enum { IN_PROGRESS, COMPLETED, CANCELED };

static int downloads;

static void report(uintptr_t page, int guid, ICoreWebView2DownloadOperation *op, const std::string &path, int state) {
	LPWSTR u = NULL;
	op->get_Uri(&u);
	std::string url = taken(u);
	INT64 received = 0, total = 0;
	op->get_BytesReceived(&received);
	op->get_TotalBytesToReceive(&total);
	std::string name = path.substr(path.find_last_of("\\/") + 1);
	lichDownload(page, guid, (char *)url.c_str(), (char *)name.c_str(), (char *)path.c_str(), state, received,
	             total < 0 ? 0 : total);
}

// A download lands where Go reserved it, the file Go made empty there so a
// second download of one name takes another; WebView2 overwrites a file at
// the path it is given.
static void connect_downloads(LichTab *tab) {
	ICoreWebView2_4 *v4 = NULL;
	if (FAILED(tab->view->QueryInterface(IID_ICoreWebView2_4, (void **)&v4)) || !v4) {
		return;
	}
	EventRegistrationToken t;
	auto h = on<ICoreWebView2DownloadStartingEventHandler>(
	    [tab](ICoreWebView2 *, ICoreWebView2DownloadStartingEventArgs *a) -> HRESULT {
		    uintptr_t page = tab->page->key;
		    ICoreWebView2DownloadOperation *started = NULL;
		    a->get_DownloadOperation(&started);
		    LPWSTR du = NULL;
		    if (started) {
			    started->get_Uri(&du);
			    started->Release();
		    }
		    std::string uri = taken(du);
		    if (!tab->pending.empty() && uri == tab->pending) {
			    if (tab->aborted) {
				    KillTimer(tab->host, ABORTED_TIMER);
				    tab->aborted = false;
			    }
			    tab->downloaded = true;
			    lichLoadFailed(tab->key, (char *)uri.c_str(), (char *)"the navigation turned into a download", 1);
		    }
		    LPWSTR def = NULL;
		    a->get_ResultFilePath(&def);
		    std::string suggested = taken(def);
		    suggested = suggested.substr(suggested.find_last_of("\\/") + 1);
		    char *reserved = lichDownloadPath(page, (char *)suggested.c_str());
		    if (!reserved) {
			    a->put_Cancel(TRUE);
			    return S_OK;
		    }
		    std::string path = reserved;
		    free(reserved);
		    std::wstring w = wide(path.c_str());
		    a->put_ResultFilePath(w.c_str());
		    a->put_Handled(TRUE);
		    ICoreWebView2DownloadOperation *op = NULL;
		    a->get_DownloadOperation(&op);
		    if (!op) {
			    return S_OK;
		    }
		    int guid = ++downloads;
		    report(page, guid, op, path, IN_PROGRESS);
		    EventRegistrationToken t;
		    auto bytes = on<ICoreWebView2BytesReceivedChangedEventHandler>(
		        [page, guid, path](ICoreWebView2DownloadOperation *op, IUnknown *) -> HRESULT {
			        report(page, guid, op, path, IN_PROGRESS);
			        return S_OK;
		        });
		    op->add_BytesReceivedChanged(bytes, &t);
		    bytes->Release();
		    auto state = on<ICoreWebView2StateChangedEventHandler>(
		        [page, guid, path](ICoreWebView2DownloadOperation *op, IUnknown *) -> HRESULT {
			        COREWEBVIEW2_DOWNLOAD_STATE s = COREWEBVIEW2_DOWNLOAD_STATE_IN_PROGRESS;
			        op->get_State(&s);
			        if (s == COREWEBVIEW2_DOWNLOAD_STATE_COMPLETED) {
				        report(page, guid, op, path, COMPLETED);
			        } else if (s == COREWEBVIEW2_DOWNLOAD_STATE_INTERRUPTED) {
				        report(page, guid, op, path, CANCELED);
			        }
			        return S_OK;
		        });
		    op->add_StateChanged(state, &t);
		    state->Release();
		    op->Release();
		    return S_OK;
	    });
	v4->add_DownloadStarting(h, &t);
	h->Release();
	v4->Release();
}

static void make_view(LichTab *tab, ICoreWebView2NewWindowRequestedEventArgs *opening, ICoreWebView2Deferral *deferral);

static void connect_tabs(LichTab *tab) {
	EventRegistrationToken t;
	auto open = on<ICoreWebView2NewWindowRequestedEventHandler>(
	    [tab](ICoreWebView2 *, ICoreWebView2NewWindowRequestedEventArgs *a) -> HRESULT {
		    uintptr_t key = lichTabCreate(tab->key);
		    if (!key) {
			    a->put_Handled(TRUE);
			    return S_OK;
		    }
		    ICoreWebView2Deferral *deferral = NULL;
		    a->GetDeferral(&deferral);
		    RECT r;
		    GetClientRect(tab->host, &r);
		    LichTab *opened = new LichTab();
		    opened->page = tab->page;
		    opened->key = key;
		    opened->scale = tab->scale;
		    opened->host = new_host(opened, r.right, r.bottom);
		    a->AddRef();
		    make_view(opened, a, deferral);
		    lichTabCreated(key, opened);
		    return S_OK;
	    });
	tab->view->add_NewWindowRequested(open, &t);
	open->Release();
	auto closing = on<ICoreWebView2WindowCloseRequestedEventHandler>([tab](ICoreWebView2 *, IUnknown *) -> HRESULT {
		lichTabClosing(tab->key);
		return S_OK;
	});
	tab->view->add_WindowCloseRequested(closing, &t);
	closing->Release();
	auto failed = on<ICoreWebView2ProcessFailedEventHandler>(
	    [tab](ICoreWebView2 *, ICoreWebView2ProcessFailedEventArgs *a) -> HRESULT {
		    COREWEBVIEW2_PROCESS_FAILED_KIND kind;
		    a->get_ProcessFailedKind(&kind);
		    if (kind == COREWEBVIEW2_PROCESS_FAILED_KIND_RENDER_PROCESS_EXITED ||
		        kind == COREWEBVIEW2_PROCESS_FAILED_KIND_BROWSER_PROCESS_EXITED ||
		        kind == COREWEBVIEW2_PROCESS_FAILED_KIND_RENDER_PROCESS_UNRESPONSIVE) {
			    lichCrashed(tab->key);
		    }
		    return S_OK;
	    });
	tab->view->add_ProcessFailed(failed, &t);
	failed->Release();
	auto message = on<ICoreWebView2WebMessageReceivedEventHandler>(
	    [tab](ICoreWebView2 *, ICoreWebView2WebMessageReceivedEventArgs *a) -> HRESULT {
		    LPWSTR json = NULL;
		    a->get_WebMessageAsJson(&json);
		    std::string s = taken(json);
		    lichMessage(tab->page->key, (char *)s.c_str());
		    return S_OK;
	    });
	tab->view->add_WebMessageReceived(message, &t);
	message->Release();
}

static void apply_scale(LichTab *tab) {
	ICoreWebView2Controller3 *c3 = NULL;
	if (SUCCEEDED(tab->controller->QueryInterface(IID_ICoreWebView2Controller3, (void **)&c3)) && c3) {
		c3->put_RasterizationScale(tab->scale);
		c3->Release();
	}
}

// ready runs what the tab was asked for while its view was made, once the
// page's script and the DevTools domains Go reads are in: a request or a
// console error of the first load is not missed.
static void ready(LichTab *tab, ICoreWebView2NewWindowRequestedEventArgs *opening, ICoreWebView2Deferral *deferral) {
	auto added = on<ICoreWebView2AddScriptToExecuteOnDocumentCreatedCompletedHandler>(
	    [tab, opening, deferral](HRESULT, LPCWSTR) -> HRESULT {
		    if (tab->closed) {
			    return S_OK;
		    }
		    cdp(tab, L"Network.enable", L"{}", [tab, opening, deferral](HRESULT, LPCWSTR) {
			    if (tab->closed) {
				    return;
			    }
			    cdp(tab, L"Page.enable", L"{}", nullptr);
			    cdp(tab, L"Page.setInterceptFileChooserDialog", L"{\"enabled\":true}", nullptr);
			    // The page acts as focused whatever window is in front, so
			    // keys and text from the wire land in it.
			    cdp(tab, L"Emulation.setFocusEmulationEnabled", L"{\"enabled\":true}", nullptr);
			    if (opening) {
				    opening->put_NewWindow(tab->view);
				    opening->put_Handled(TRUE);
				    deferral->Complete();
				    deferral->Release();
				    opening->Release();
			    }
			    tab->ready = true;
			    std::vector<std::function<void()>> later;
			    later.swap(tab->later);
			    for (auto &f : later) {
				    f();
			    }
		    });
		    return S_OK;
	    });
	tab->view->AddScriptToExecuteOnDocumentCreated(tab->page->script.c_str(), added);
	added->Release();
}

static void made(LichTab *tab, ICoreWebView2Controller *c, ICoreWebView2NewWindowRequestedEventArgs *opening,
                 ICoreWebView2Deferral *deferral) {
	tab->controller = c;
	c->AddRef();
	c->get_CoreWebView2(&tab->view);
	ICoreWebView2Controller3 *c3 = NULL;
	if (SUCCEEDED(c->QueryInterface(IID_ICoreWebView2Controller3, (void **)&c3)) && c3) {
		c3->put_BoundsMode(COREWEBVIEW2_BOUNDS_MODE_USE_RAW_PIXELS);
		c3->put_ShouldDetectMonitorScaleChanges(FALSE);
		c3->Release();
	}
	apply_scale(tab);
	fit(tab, tab->host);
	c->put_IsVisible(TRUE);
	ICoreWebView2Settings *s = NULL;
	if (SUCCEEDED(tab->view->get_Settings(&s)) && s) {
		s->put_AreDefaultScriptDialogsEnabled(FALSE);
		s->put_AreDevToolsEnabled(TRUE);
		s->put_IsStatusBarEnabled(FALSE);
		s->Release();
	}
	connect_loads(tab);
	connect_icon(tab);
	connect_dialogs(tab);
	connect_downloads(tab);
	connect_tabs(tab);
	listen_cdp(tab);
	ready(tab, opening, deferral);
}

static void make_view(LichTab *tab, ICoreWebView2NewWindowRequestedEventArgs *opening, ICoreWebView2Deferral *deferral) {
	auto done = on<ICoreWebView2CreateCoreWebView2ControllerCompletedHandler>(
	    [tab, opening, deferral](HRESULT hr, ICoreWebView2Controller *c) -> HRESULT {
		    if (tab->closed) {
			    if (SUCCEEDED(hr) && c) {
				    c->Close();
			    }
			    return S_OK;
		    }
		    if (FAILED(hr) || !c) {
			    fprintf(stderr, "lich-browser: WebView2 made no view for tab %llu (HRESULT 0x%08lx)\n",
			            (unsigned long long)tab->key, (unsigned long)hr);
			    lichCrashed(tab->key);
			    return S_OK;
		    }
		    made(tab, c, opening, deferral);
		    return S_OK;
	    });
	ICoreWebView2Environment10 *env10 = NULL;
	ICoreWebView2ControllerOptions *options = NULL;
	if (SUCCEEDED(env->QueryInterface(IID_ICoreWebView2Environment10, (void **)&env10)) && env10 &&
	    SUCCEEDED(env10->CreateCoreWebView2ControllerOptions(&options)) && options) {
		options->put_ProfileName(tab->page->profile.c_str());
		options->put_IsInPrivateModeEnabled(TRUE);
		env10->CreateCoreWebView2ControllerWithOptions(tab->host, options, done);
		options->Release();
		env10->Release();
	} else {
		env->CreateCoreWebView2Controller(tab->host, done);
	}
	done->Release();
}

LichTab *lich_tab_new(LichPage *page, void *related, uintptr_t key, int width, int height) {
	LichTab *tab = new LichTab();
	tab->page = page;
	tab->key = key;
	tab->scale = 1;
	tab->host = new_host(tab, width, height);
	make_view(tab, NULL, NULL);
	return tab;
}

uint64_t lich_tab_xid(LichTab *tab) { return (uint64_t)(uintptr_t)tab->host; }

void lich_tab_close(LichTab *tab) {
	tab->closed = true;
	if (tab->inspector) {
		KillTimer(tab->host, DEVTOOLS_TIMER);
		if (tab->devtools && IsWindow(tab->devtools)) {
			// DevTools' window is the browser process's: it leaves the
			// tab's window before that one goes, and closes itself.
			SetParent(tab->devtools, NULL);
			PostMessageW(tab->devtools, WM_CLOSE, 0, 0);
		}
	}
	if (tab->controller) {
		tab->controller->Close();
		tab->controller->Release();
		tab->controller = NULL;
	}
	if (tab->view) {
		tab->view->Release();
		tab->view = NULL;
	}
	SetWindowLongPtrW(tab->host, GWLP_USERDATA, 0);
	DestroyWindow(tab->host);
	// shortcut: a closed tab's struct is never freed, since a WebView2
	// callback already queued may still read it; a few hundred bytes a tab,
	// until lich-browser ends with its last page.
}

// embedded is whether the window holds the tab's window: then the window
// sizes it.
static bool embedded(LichTab *tab) { return GetWindowLongPtrW(tab->host, GWL_STYLE) & WS_CHILD; }

void lich_tab_resize(LichTab *tab, int width, int height) {
	if (!embedded(tab)) {
		SetWindowPos(tab->host, NULL, 0, 0, width, height, SWP_NOMOVE | SWP_NOZORDER | SWP_NOACTIVATE);
	}
}

void lich_tab_zoom(LichTab *tab, double scale) {
	tab->scale = scale;
	when(tab, [tab] { apply_scale(tab); });
}

void lich_tab_navigate(LichTab *tab, const char *url) {
	std::wstring u = wide(url);
	when(tab, [tab, u] { tab->view->Navigate(u.c_str()); });
}

void lich_tab_reload(LichTab *tab) {
	// Past the cache, as the contract asks, which Reload is not.
	when(tab, [tab] { cdp(tab, L"Page.reload", L"{\"ignoreCache\":true}", nullptr); });
}

void lich_tab_back(LichTab *tab) {
	when(tab, [tab] { tab->view->GoBack(); });
}

void lich_tab_forward(LichTab *tab) {
	when(tab, [tab] { tab->view->GoForward(); });
}

void lich_tab_run(LichTab *tab, const char *script) {
	std::wstring s = wide(script);
	when(tab, [tab, s] { tab->view->ExecuteScript(s.c_str(), NULL); });
}

void lich_tab_draw(LichTab *tab, const char *script, uintptr_t key) {
	std::wstring s = wide(script);
	when(tab, [tab, s, key] {
		auto done = on<ICoreWebView2ExecuteScriptCompletedHandler>([key](HRESULT, LPCWSTR) -> HRESULT {
			lichDrawn(key);
			return S_OK;
		});
		tab->view->ExecuteScript(s.c_str(), done);
		done->Release();
	});
}

void lich_tab_cdp(LichTab *tab, const char *method, const char *params, uintptr_t handle) {
	std::wstring m = wide(method), p = wide(params);
	if (tab->closed) {
		lichCDPDone(handle, 0, (char *)"{\"message\":\"the tab closed\"}");
		return;
	}
	when(tab, [tab, m, p, handle] {
		cdp(tab, m.c_str(), p.c_str(), [handle](HRESULT hr, LPCWSTR json) {
			std::string s = narrow(json);
			lichCDPDone(handle, SUCCEEDED(hr), (char *)s.c_str());
		});
	});
}

const char *lich_tab_uri(LichTab *tab) { return tab->uri.c_str(); }

const char *lich_tab_title(LichTab *tab) { return tab->title.c_str(); }

int lich_tab_loading(LichTab *tab) { return tab->loading; }

unsigned char *lich_tab_icon(LichTab *tab, int *length) {
	*length = (int)tab->icon.size();
	if (tab->icon.empty()) {
		return NULL;
	}
	unsigned char *png = (unsigned char *)malloc(tab->icon.size());
	memcpy(png, tab->icon.data(), tab->icon.size());
	return png;
}

// The inspector: WebView2 opens DevTools only as a window of its browser
// process's own. The tab's inspector is that window, found among the
// browser process's windows once it appears and made a child of a tab
// window of lich-browser's, which the pane embeds as any tab.

struct Search {
	DWORD pid;
	HWND found;
};

static BOOL CALLBACK find_window(HWND w, LPARAM lp) {
	Search *s = (Search *)lp;
	DWORD pid = 0;
	GetWindowThreadProcessId(w, &pid);
	if (pid != s->pid || !IsWindowVisible(w)) {
		return TRUE;
	}
	wchar_t title[64];
	GetWindowTextW(w, title, 64);
	if (wcsncmp(title, L"DevTools", 8) == 0) {
		s->found = w;
		return FALSE;
	}
	return TRUE;
}

static void find_devtools(LichTab *d) {
	LichTab *t = d->inspected;
	if (d->devtools) {
		// Closed by the person, from its own window.
		if (!IsWindow(d->devtools)) {
			KillTimer(d->host, DEVTOOLS_TIMER);
			d->devtools = NULL;
			lichDevtoolsClosed(t->key);
		}
		return;
	}
	UINT32 pid = 0;
	t->view->get_BrowserProcessId(&pid);
	Search s = {pid, NULL};
	EnumWindows(find_window, (LPARAM)&s);
	if (!s.found) {
		if (++d->polls >= DEVTOOLS_POLLS) {
			KillTimer(d->host, DEVTOOLS_TIMER);
			fprintf(stderr, "lich-browser: DevTools opened no window to hold\n");
		}
		return;
	}
	d->devtools = s.found;
	LONG_PTR style = GetWindowLongPtrW(s.found, GWL_STYLE);
	style = (style & ~(WS_POPUP | WS_CAPTION | WS_THICKFRAME | WS_SYSMENU | WS_MINIMIZEBOX | WS_MAXIMIZEBOX)) | WS_CHILD;
	SetWindowLongPtrW(s.found, GWL_STYLE, style);
	SetParent(s.found, d->host);
	fit(d, d->host);
	ShowWindow(s.found, SW_SHOWNOACTIVATE);
	// Keep the timer, now to tell a DevTools window that closed.
	lichDevtoolsOpened(d->key, d);
}

void lich_tab_inspect(LichTab *tab) {
	when(tab, [tab] {
		uintptr_t key = lichDevtoolsKey(tab->key);
		if (!key) {
			return;
		}
		RECT r;
		GetClientRect(tab->host, &r);
		LichTab *d = new LichTab();
		d->page = tab->page;
		d->key = key;
		d->inspector = true;
		d->inspected = tab;
		d->ready = true;
		d->host = new_host(d, r.right, r.bottom);
		tab->view->OpenDevToolsWindow();
		SetTimer(d->host, DEVTOOLS_TIMER, DEVTOOLS_POLL_MS, NULL);
	});
}

void lich_tab_close_inspector(LichTab *tab) {
	// Go drops the inspector's tab right after, closing its window
	// (lich_tab_close), which ends DevTools.
	(void)tab;
}

// The person's own window for a page: the front tab's view moves into it,
// the same view, so its page goes on as it was.

static void move_view(LichTab *tab, HWND to) {
	if (tab->inspector) {
		if (tab->devtools) {
			SetParent(tab->devtools, to);
		}
	} else {
		when(tab, [tab, to] { tab->controller->put_ParentWindow(to); });
	}
	fit(tab, to);
}

void lich_page_pop_out(LichPage *page, LichTab *tab, int width, int height) {
	page->popped = CreateWindowExW(0, POPPED_CLASS, L"lich", WS_OVERLAPPEDWINDOW | WS_CLIPCHILDREN, CW_USEDEFAULT,
	                               CW_USEDEFAULT, width, height, NULL, NULL, GetModuleHandleW(NULL), NULL);
	SetWindowLongPtrW(page->popped, GWLP_USERDATA, (LONG_PTR)page);
	page->shown = tab;
	move_view(tab, page->popped);
	ShowWindow(page->popped, SW_SHOW);
	fit(tab, page->popped);
}

void lich_page_show(LichPage *page, LichTab *tab) {
	if (page->shown == tab) {
		return;
	}
	if (page->shown) {
		move_view(page->shown, page->shown->host);
	}
	page->shown = tab;
	move_view(tab, page->popped);
}

void lich_page_pop_in(LichPage *page) {
	LichTab *tab = page->shown;
	move_view(tab, tab->host);
	SetWindowLongPtrW(page->popped, GWLP_USERDATA, 0);
	DestroyWindow(page->popped);
	page->popped = NULL;
	page->shown = NULL;
}

void lich_page_raise(LichPage *page) {
	ShowWindow(page->popped, SW_RESTORE);
	SetForegroundWindow(page->popped);
}
