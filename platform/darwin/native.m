//go:build darwin && cgo && vitra_native

#import "native.h"
#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#import <stdlib.h>
#import <string.h>

extern void goVitraIdle(void *);
extern void goVitraMessage(char *, char *);
extern void goVitraDestroy(char *);
extern int goVitraNav(char *, char *);

@interface VitraWinDelegate : NSObject <WKScriptMessageHandler, WKNavigationDelegate, NSWindowDelegate> {
	char *windowID;
}
- (instancetype)initWithWindowID:(char *)wid;
@end

@implementation VitraWinDelegate

- (instancetype)initWithWindowID:(char *)wid {
	self = [super init];
	if (self) {
		windowID = wid;
	}
	return self;
}

- (void)userContentController:(WKUserContentController *)userContentController
      didReceiveScriptMessage:(WKScriptMessage *)message {
	(void)userContentController;
	NSString *body = nil;
	if ([message.body isKindOfClass:[NSString class]]) {
		body = (NSString *)message.body;
	} else if ([message.body isKindOfClass:[NSDictionary class]] ||
		   [message.body isKindOfClass:[NSArray class]]) {
		NSData *data = [NSJSONSerialization dataWithJSONObject:message.body options:0 error:nil];
		if (data) {
			body = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
		}
	}
	if (!body || !windowID) {
		return;
	}
	goVitraMessage(windowID, (char *)[body UTF8String]);
}

- (void)webView:(WKWebView *)webView
    decidePolicyForNavigationAction:(WKNavigationAction *)navigationAction
		    decisionHandler:(void (^)(WKNavigationActionPolicy))decisionHandler {
	(void)webView;
	NSURL *url = navigationAction.request.URL;
	const char *uri = url ? [url.absoluteString UTF8String] : "";
	if (goVitraNav(windowID, (char *)uri)) {
		decisionHandler(WKNavigationActionPolicyAllow);
	} else {
		decisionHandler(WKNavigationActionPolicyCancel);
	}
}

- (void)windowWillClose:(NSNotification *)notification {
	(void)notification;
	if (windowID) {
		goVitraDestroy(windowID);
	}
}

@end

struct VitraWin {
	NSWindow *window;
	WKWebView *view;
	VitraWinDelegate *delegate;
	char *id;
	int maximized;
	int fullscreen;
	int above;
	int minimized;
	int hidden;
	int req_width;
	int req_height;
	char *icon_path;
};

void vitra_app_init(void) {
	[NSApplication sharedApplication];
	[NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
}

void vitra_app_run(void) {
	[NSApp run];
}

void vitra_app_quit(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		[NSApp stop:nil];
		NSEvent *ev = [NSEvent otherEventWithType:NSEventTypeApplicationDefined
						 location:NSZeroPoint
					    modifierFlags:0
						timestamp:0
					     windowNumber:0
						  context:nil
						  subtype:0
						    data1:0
						    data2:0];
		[NSApp postEvent:ev atStart:YES];
	});
}

void vitra_idle_add(void *data) {
	dispatch_async(dispatch_get_main_queue(), ^{
		goVitraIdle(data);
	});
}

VitraWin *vitra_win_new(const char *id, const char *title, int width, int height, const char *uri, const char *preload) {
	VitraWin *w = (VitraWin *)calloc(1, sizeof(VitraWin));
	if (!w) {
		return NULL;
	}
	w->id = strdup(id ? id : "");
	w->req_width = width > 0 ? width : 1024;
	w->req_height = height > 0 ? height : 768;
	w->delegate = [[VitraWinDelegate alloc] initWithWindowID:w->id];

	WKUserContentController *ucc = [[WKUserContentController alloc] init];
	[ucc addScriptMessageHandler:w->delegate name:@"vitra"];
	if (preload && preload[0] != '\0') {
		NSString *src = [NSString stringWithUTF8String:preload];
		WKUserScript *script = [[WKUserScript alloc]
		    initWithSource:src
			 injectionTime:WKUserScriptInjectionTimeAtDocumentStart
		      forMainFrameOnly:YES];
		[ucc addUserScript:script];
		[script release];
	}

	WKWebViewConfiguration *config = [[WKWebViewConfiguration alloc] init];
	config.userContentController = ucc;
	[ucc release];

	NSRect frame = NSMakeRect(0, 0, width > 0 ? width : 1024, height > 0 ? height : 768);
	w->view = [[WKWebView alloc] initWithFrame:frame configuration:config];
	[config release];
	w->view.navigationDelegate = w->delegate;

	NSUInteger style = NSWindowStyleMaskTitled | NSWindowStyleMaskClosable |
			   NSWindowStyleMaskMiniaturizable | NSWindowStyleMaskResizable;
	w->window = [[NSWindow alloc] initWithContentRect:frame
					    styleMask:style
					      backing:NSBackingStoreBuffered
						defer:NO];
	w->window.title = title ? [NSString stringWithUTF8String:title] : @"";
	w->window.contentView = w->view;
	w->window.delegate = w->delegate;
	[w->window center];
	[w->window makeKeyAndOrderFront:nil];
	[NSApp activateIgnoringOtherApps:YES];

	if (uri && uri[0] != '\0') {
		NSURL *url = [NSURL URLWithString:[NSString stringWithUTF8String:uri]];
		if (url) {
			[w->view loadRequest:[NSURLRequest requestWithURL:url]];
		}
	}
	return w;
}

void vitra_win_navigate(VitraWin *w, const char *uri) {
	if (!w || !w->view || !uri) {
		return;
	}
	NSURL *url = [NSURL URLWithString:[NSString stringWithUTF8String:uri]];
	if (url) {
		[w->view loadRequest:[NSURLRequest requestWithURL:url]];
	}
}

void vitra_win_eval(VitraWin *w, const char *js) {
	if (!w || !w->view || !js) {
		return;
	}
	NSString *script = [NSString stringWithUTF8String:js];
	[w->view evaluateJavaScript:script completionHandler:nil];
}

void vitra_win_close(VitraWin *w) {
	if (!w || !w->window) {
		return;
	}
	/* Triggers windowWillClose → goVitraDestroy. Caller frees after map removal. */
	[w->window close];
}

void vitra_win_free(VitraWin *w) {
	if (!w) {
		return;
	}
	if (w->window) {
		w->window.delegate = nil;
	}
	if (w->view) {
		[w->view.configuration.userContentController removeScriptMessageHandlerForName:@"vitra"];
		w->view.navigationDelegate = nil;
		[w->view release];
		w->view = nil;
	}
	if (w->window) {
		[w->window release];
		w->window = nil;
	}
	if (w->delegate) {
		[w->delegate release];
		w->delegate = nil;
	}
	free(w->icon_path);
	free(w->id);
	free(w);
}

void vitra_win_apply_chrome(VitraWin *w, const char *title, int width, int height, int maximized, int fullscreen, int above, int minimized, int hidden, const char *icon_path) {
	if (!w || !w->window) {
		return;
	}
	NSWindow *win = w->window;
	if (title) {
		win.title = [NSString stringWithUTF8String:title];
	}
	if (width > 0 && height > 0) {
		w->req_width = width;
		w->req_height = height;
		NSRect frame = win.frame;
		NSRect content = [win contentRectForFrameRect:frame];
		content.size.width = width;
		content.size.height = height;
		NSRect newFrame = [win frameRectForContentRect:content];
		newFrame.origin = frame.origin;
		[win setFrame:newFrame display:YES animate:NO];
	}
	w->maximized = maximized ? 1 : 0;
	w->fullscreen = fullscreen ? 1 : 0;
	w->above = above ? 1 : 0;
	w->minimized = minimized ? 1 : 0;
	w->hidden = hidden ? 1 : 0;

	BOOL isZoomed = win.zoomed;
	if (maximized && !isZoomed) {
		[win zoom:nil];
	} else if (!maximized && isZoomed) {
		[win zoom:nil];
	}

	BOOL isFull = (win.styleMask & NSWindowStyleMaskFullScreen) != 0;
	if (fullscreen && !isFull) {
		[win toggleFullScreen:nil];
	} else if (!fullscreen && isFull) {
		[win toggleFullScreen:nil];
	}

	[win setLevel:(above ? NSFloatingWindowLevel : NSNormalWindowLevel)];

	if (minimized) {
		[win miniaturize:nil];
	} else if (win.miniaturized) {
		[win deminiaturize:nil];
	}

	if (hidden) {
		[win orderOut:nil];
	} else {
		[win makeKeyAndOrderFront:nil];
	}

	if (icon_path && icon_path[0] != '\0') {
		NSImage *img = [[NSImage alloc] initWithContentsOfFile:[NSString stringWithUTF8String:icon_path]];
		if (img) {
			win.miniwindowImage = img;
			[img release];
			free(w->icon_path);
			w->icon_path = strdup(icon_path);
		}
	}
}

VitraChrome vitra_win_chrome(VitraWin *w) {
	VitraChrome c;
	memset(&c, 0, sizeof(c));
	c.title = strdup("");
	c.icon_path = strdup("");
	if (!w || !w->window) {
		return c;
	}
	NSWindow *win = w->window;
	free(c.title);
	const char *t = win.title ? [win.title UTF8String] : "";
	c.title = strdup(t ? t : "");
	free(c.icon_path);
	c.icon_path = strdup(w->icon_path ? w->icon_path : "");
	NSRect content = [win contentRectForFrameRect:win.frame];
	c.width = (int)content.size.width;
	c.height = (int)content.size.height;
	if (c.width <= 0) {
		c.width = w->req_width;
	}
	if (c.height <= 0) {
		c.height = w->req_height;
	}
	c.maximized = win.zoomed ? 1 : 0;
	c.fullscreen = (win.styleMask & NSWindowStyleMaskFullScreen) ? 1 : 0;
	c.above = (win.level >= NSFloatingWindowLevel) ? 1 : 0;
	c.minimized = win.miniaturized ? 1 : 0;
	c.hidden = (!win.visible) ? 1 : 0;
	return c;
}

char *vitra_open_dialog(void) {
	NSOpenPanel *panel = [NSOpenPanel openPanel];
	panel.canChooseFiles = YES;
	panel.canChooseDirectories = NO;
	panel.allowsMultipleSelection = NO;
	panel.resolvesAliases = YES;
	if ([panel runModal] != NSModalResponseOK) {
		return NULL;
	}
	NSURL *url = panel.URL;
	if (!url || !url.path) {
		return NULL;
	}
	return strdup(url.fileSystemRepresentation);
}

char *vitra_save_dialog(void) {
	NSSavePanel *panel = [NSSavePanel savePanel];
	panel.canCreateDirectories = YES;
	if ([panel runModal] != NSModalResponseOK) {
		return NULL;
	}
	NSURL *url = panel.URL;
	if (!url || !url.path) {
		return NULL;
	}
	return strdup(url.fileSystemRepresentation);
}
