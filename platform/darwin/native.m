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
	free(w->id);
	free(w);
}
