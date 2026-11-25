#include "browser_linux.h"

#include <glib/gstdio.h>
#include <string.h>

extern void lichLoad(uintptr_t key, int step, char *uri);
extern void lichLoadFailed(uintptr_t key, char *uri);
extern void lichTLS(uintptr_t key, char *uri, char *chain);
extern void lichTabChanged(uintptr_t key, int icon);
extern uintptr_t lichTabCreate(uintptr_t parent);
extern void lichTabCreated(uintptr_t key, LichTab *tab);
extern void lichTabClosing(uintptr_t key);
extern int lichRequest(uintptr_t key, int started);
extern void lichCrashed(uintptr_t key);
extern int lichDialog(uintptr_t key, WebKitScriptDialog *dialog, int type, char *message, char *text);
extern void lichFiles(uintptr_t key, WebKitFileChooserRequest *request, int multiple);
extern void lichConsole(uintptr_t page, char *text);
extern void lichInputSeen(uintptr_t page, char *type);
extern void lichDownload(uintptr_t page, int guid, char *url, char *name, char *path, int state, gint64 received,
                         gint64 total);

static void load_changed(WebKitWebView *view, WebKitLoadEvent step, gpointer tab) {
	lichLoad(((LichTab *)tab)->key, step, (char *)webkit_web_view_get_uri(view));
}

static gboolean load_failed(WebKitWebView *view, WebKitLoadEvent step, char *uri, GError *error, gpointer tab) {
	lichLoadFailed(((LichTab *)tab)->key, uri);
	return FALSE;
}

// tls_failed hands Go the chain of a server whose root WebKit cannot find,
// mostly a server that left out its intermediate (see completeChain); WebKit
// shows its error page meanwhile.
static gboolean tls_failed(WebKitWebView *view, char *uri, GTlsCertificate *cert, GTlsCertificateFlags errors,
                           gpointer tab) {
	lichLoadFailed(((LichTab *)tab)->key, uri);
	if (errors != G_TLS_CERTIFICATE_UNKNOWN_CA) {
		return FALSE;
	}
	GString *chain = g_string_new(NULL);
	for (GTlsCertificate *c = cert; c; c = g_tls_certificate_get_issuer(c)) {
		char *pem = NULL;
		g_object_get(c, "certificate-pem", &pem, NULL);
		if (pem) {
			g_string_append(chain, pem);
			g_free(pem);
		}
	}
	lichTLS(((LichTab *)tab)->key, uri, chain->str);
	g_string_free(chain, TRUE);
	return FALSE;
}

static void changed(GObject *view, GParamSpec *spec, gpointer tab) { lichTabChanged(((LichTab *)tab)->key, 0); }

static void icon_changed(GObject *view, GParamSpec *spec, gpointer tab) { lichTabChanged(((LichTab *)tab)->key, 1); }

// create makes the tab a page opens with window.open or a target=_blank
// link, related to its opener so the two share a process and can script
// each other.
static GtkWidget *create(WebKitWebView *view, WebKitNavigationAction *action, gpointer data) {
	LichTab *opener = data;
	uintptr_t key = lichTabCreate(opener->key);
	if (!key) {
		return NULL;
	}
	LichTab *tab = lich_tab_new(opener->page, view, key, opener->width, opener->height);
	lichTabCreated(key, tab);
	return tab->view;
}

static void closing(WebKitWebView *view, gpointer tab) { lichTabClosing(((LichTab *)tab)->key); }

static void resource_finished(WebKitWebResource *resource, gpointer key) { lichRequest((uintptr_t)key, 0); }

static void resource_started(WebKitWebView *view, WebKitWebResource *resource, WebKitURIRequest *request,
                             gpointer data) {
	LichTab *tab = data;
	if (resource == webkit_web_view_get_main_resource(view) || !lichRequest(tab->key, 1)) {
		return;
	}
	g_signal_connect(resource, "finished", G_CALLBACK(resource_finished), (gpointer)tab->key);
}

static void terminated(WebKitWebView *view, WebKitWebProcessTerminationReason reason, gpointer tab) {
	lichCrashed(((LichTab *)tab)->key);
}

// A dialog is answered later, from the wire: Go keeps a reference until then.
static gboolean script_dialog(WebKitWebView *view, WebKitScriptDialog *dialog, gpointer tab) {
	int type = webkit_script_dialog_get_dialog_type(dialog);
	const char *text = type == WEBKIT_SCRIPT_DIALOG_PROMPT ? webkit_script_dialog_prompt_get_default_text(dialog) : "";
	lichDialog(((LichTab *)tab)->key, dialog, type, (char *)webkit_script_dialog_get_message(dialog), (char *)text);
	return TRUE;
}

static gboolean file_chooser(WebKitWebView *view, WebKitFileChooserRequest *request, gpointer tab) {
	lichFiles(((LichTab *)tab)->key, request, webkit_file_chooser_request_get_select_multiple(request));
	return TRUE;
}

void lich_events_connect_tab(LichTab *tab) {
	GtkWidget *v = tab->view;
	g_signal_connect(v, "load-changed", G_CALLBACK(load_changed), tab);
	g_signal_connect(v, "load-failed", G_CALLBACK(load_failed), tab);
	g_signal_connect(v, "load-failed-with-tls-errors", G_CALLBACK(tls_failed), tab);
	g_signal_connect(v, "notify::uri", G_CALLBACK(changed), tab);
	g_signal_connect(v, "notify::title", G_CALLBACK(changed), tab);
	g_signal_connect(v, "notify::is-loading", G_CALLBACK(changed), tab);
	g_signal_connect(v, "notify::favicon", G_CALLBACK(icon_changed), tab);
	g_signal_connect(v, "create", G_CALLBACK(create), tab);
	g_signal_connect(v, "close", G_CALLBACK(closing), tab);
	g_signal_connect(v, "resource-load-started", G_CALLBACK(resource_started), tab);
	g_signal_connect(v, "web-process-terminated", G_CALLBACK(terminated), tab);
	g_signal_connect(v, "script-dialog", G_CALLBACK(script_dialog), tab);
	g_signal_connect(v, "run-file-chooser", G_CALLBACK(file_chooser), tab);
}

void lich_dialog_answer(WebKitScriptDialog *dialog, int accept, const char *text) {
	switch (webkit_script_dialog_get_dialog_type(dialog)) {
	case WEBKIT_SCRIPT_DIALOG_CONFIRM:
	case WEBKIT_SCRIPT_DIALOG_BEFORE_UNLOAD_CONFIRM:
		webkit_script_dialog_confirm_set_confirmed(dialog, accept);
		break;
	case WEBKIT_SCRIPT_DIALOG_PROMPT:
		// Left unset, the prompt answers null, a dismissal.
		if (accept) {
			webkit_script_dialog_prompt_set_text(dialog, text);
		}
		break;
	default:
		break;
	}
	webkit_script_dialog_close(dialog);
	webkit_script_dialog_unref(dialog);
}

void lich_files_answer(WebKitFileChooserRequest *request, const char **paths, int n) {
	if (n == 0) {
		webkit_file_chooser_request_cancel(request);
	} else {
		webkit_file_chooser_request_select_files(request, paths);
	}
	g_object_unref(request);
}

static void console_message(WebKitUserContentManager *content, WebKitJavascriptResult *result, gpointer page) {
	char *text = jsc_value_to_string(webkit_javascript_result_get_js_value(result));
	lichConsole(((LichPage *)page)->key, text);
	g_free(text);
}

static void input_seen(WebKitUserContentManager *content, WebKitJavascriptResult *result, gpointer page) {
	char *type = jsc_value_to_string(webkit_javascript_result_get_js_value(result));
	lichInputSeen(((LichPage *)page)->key, type);
	g_free(type);
}

// fetch is one download: its page, its id on the wire, and where it lands.
typedef struct {
	uintptr_t page;
	int guid;
	char *name;
	char *path;
	int failed;
} Fetch;

enum { IN_PROGRESS, COMPLETED, CANCELED };

static char *downloads_dir;
static int downloads;

void lich_browser_set_downloads(const char *dir) { downloads_dir = g_strdup(dir); }

static void fetch_free(gpointer data) {
	Fetch *f = data;
	g_free(f->name);
	g_free(f->path);
	g_free(f);
}

static void report(WebKitDownload *d, Fetch *f, int state) {
	WebKitURIResponse *response = webkit_download_get_response(d);
	gint64 total = response ? (gint64)webkit_uri_response_get_content_length(response) : 0;
	const char *url = webkit_uri_request_get_uri(webkit_download_get_request(d));
	lichDownload(f->page, f->guid, (char *)url, f->name ? f->name : "", f->path ? f->path : "", state,
	             (gint64)webkit_download_get_received_data_length(d), total);
}

// decide puts each download in a folder of its own, so two of one name
// both land; the backend moves it where the person finds it.
static gboolean decide(WebKitDownload *d, char *suggested, gpointer data) {
	Fetch *f = data;
	char *dir = g_strdup_printf("%s/%d", downloads_dir, f->guid);
	g_mkdir_with_parents(dir, 0700);
	f->name = g_path_get_basename(suggested && *suggested ? suggested : "download");
	f->path = g_build_filename(dir, f->name, NULL);
	char *uri = g_filename_to_uri(f->path, NULL, NULL);
	webkit_download_set_destination(d, uri);
	g_free(uri);
	g_free(dir);
	report(d, f, IN_PROGRESS);
	return TRUE;
}

static void received(WebKitDownload *d, guint64 length, gpointer f) { report(d, f, IN_PROGRESS); }

static void failed(WebKitDownload *d, GError *error, gpointer data) {
	Fetch *f = data;
	f->failed = 1;
	report(d, f, CANCELED);
}

// finished comes after failed too.
static void finished(WebKitDownload *d, gpointer data) {
	Fetch *f = data;
	if (!f->failed) {
		report(d, f, COMPLETED);
	}
}

static void download_started(WebKitWebContext *context, WebKitDownload *d, gpointer page) {
	Fetch *f = g_new0(Fetch, 1);
	f->page = ((LichPage *)page)->key;
	f->guid = ++downloads;
	g_object_set_data_full(G_OBJECT(d), "lich-fetch", f, fetch_free);
	g_signal_connect(d, "decide-destination", G_CALLBACK(decide), f);
	g_signal_connect(d, "received-data", G_CALLBACK(received), f);
	g_signal_connect(d, "failed", G_CALLBACK(failed), f);
	g_signal_connect(d, "finished", G_CALLBACK(finished), f);
}

void lich_events_connect_page(LichPage *page) {
	webkit_user_content_manager_register_script_message_handler(page->content, "lich");
	g_signal_connect(page->content, "script-message-received::lich", G_CALLBACK(console_message), page);
	webkit_user_content_manager_register_script_message_handler(page->content, "lichInput");
	g_signal_connect(page->content, "script-message-received::lichInput", G_CALLBACK(input_seen), page);
	g_signal_connect(page->context, "download-started", G_CALLBACK(download_started), page);
}
