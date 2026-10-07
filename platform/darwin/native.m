//go:build darwin && cgo && vitra_native

#import "native.h"
#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#import <UniformTypeIdentifiers/UniformTypeIdentifiers.h>
#import <UserNotifications/UserNotifications.h>
#import <Carbon/Carbon.h>
#import <ServiceManagement/ServiceManagement.h>
#import <stdlib.h>
#import <string.h>
#import <ctype.h>
#import <limits.h>

/* Web inspector for newly created webviews; off unless the app opts in. */
static int vitra_devtools = 0;

void vitra_set_devtools(int enabled) { vitra_devtools = enabled; }

extern void goVitraIdle(unsigned long long);
extern void goVitraMessage(char *, char *, char *);
extern void goVitraReject(char *, char *);
extern void goVitraDestroy(char *);
extern int goVitraNav(char *, char *);
extern void goVitraAction(char *);
extern void goVitraDrop(char *, char *);
extern void goVitraTrayClick(int, int, int, int, int);

@interface VitraWinDelegate : NSObject <WKScriptMessageHandler, WKNavigationDelegate, NSWindowDelegate> {
	char *windowID;
}
- (instancetype)initWithWindowID:(char *)wid;
@end

@interface VitraMenuTarget : NSObject
- (void)onAction:(id)sender;
- (void)onTrayClick:(id)sender;
@end

static void vitra_tray_show_menu(void);

@implementation VitraMenuTarget
- (void)onTrayClick:(id)sender {
	(void)sender;
	NSEvent *ev = NSApp.currentEvent;
	BOOL secondary = ev && (ev.type == NSEventTypeRightMouseUp || ev.type == NSEventTypeRightMouseDown ||
				(ev.modifierFlags & NSEventModifierFlagControl));
	if (secondary) {
		vitra_tray_show_menu();
		return;
	}
	int x = 0, y = 0, w = 0, h = 0;
	int has = vitra_tray_anchor(&x, &y, &w, &h);
	goVitraTrayClick(x, y, w, h, has);
}

- (void)onAction:(id)sender {
	NSMenuItem *item = (NSMenuItem *)sender;
	NSString *actionID = item.representedObject;
	if ([actionID isKindOfClass:[NSString class]] && actionID.length > 0) {
		goVitraAction((char *)[actionID UTF8String]);
	}
}
@end

@interface VitraDropView : NSView {
	char *windowID;
	int dropEnabled;
}
- (instancetype)initWithFrame:(NSRect)frame windowID:(char *)wid;
- (void)setDropEnabled:(int)enabled;
@end

@implementation VitraDropView

- (instancetype)initWithFrame:(NSRect)frame windowID:(char *)wid {
	self = [super initWithFrame:frame];
	if (self) {
		windowID = wid;
		dropEnabled = 0;
		self.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
	}
	return self;
}

- (void)setDropEnabled:(int)enabled {
	dropEnabled = enabled ? 1 : 0;
	if (dropEnabled) {
		[self registerForDraggedTypes:@[NSPasteboardTypeFileURL]];
	} else {
		[self unregisterDraggedTypes];
	}
}

- (NSDragOperation)draggingEntered:(id<NSDraggingInfo>)sender {
	(void)sender;
	return dropEnabled ? NSDragOperationCopy : NSDragOperationNone;
}

- (BOOL)performDragOperation:(id<NSDraggingInfo>)sender {
	if (!dropEnabled || !windowID) {
		return NO;
	}
	NSPasteboard *pb = [sender draggingPasteboard];
	NSArray<NSURL *> *urls = [pb readObjectsForClasses:@[[NSURL class]]
					   options:@{NSPasteboardURLReadingFileURLsOnlyKey: @YES}];
	if (urls.count == 0) {
		return NO;
	}
	NSMutableString *joined = [NSMutableString string];
	for (NSURL *url in urls) {
		NSString *path = url.path;
		if (path.length == 0) {
			continue;
		}
		if (joined.length > 0) {
			[joined appendString:@"\n"];
		}
		[joined appendString:path];
	}
	if (joined.length == 0) {
		return NO;
	}
	goVitraDrop(windowID, (char *)[joined UTF8String]);
	return YES;
}

@end

/* VitraPanel is a borderless tray panel that can still take keyboard focus
 * (borderless windows cannot by default). hiddenAt is when it last hid
 * itself on losing key status. */
@interface VitraPanel : NSPanel
@property(nonatomic) CFAbsoluteTime hiddenAt;
@end

@implementation VitraPanel
- (BOOL)canBecomeKeyWindow {
	return YES;
}
- (BOOL)canBecomeMainWindow {
	return NO;
}
@end

/* A click on the tray icon that took the panel's focus arrives this long
 * after the panel hid itself; it means "close", not "open again". */
static const CFAbsoluteTime kVitraPanelReopenGuard = 0.3;

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
	/* Only the top frame carries the bridge; drop anything a subframe posts
	 * (the host also requires the top frame's sender token). */
	if (!message.frameInfo.isMainFrame) {
		if (windowID) {
			goVitraReject(windowID, "message from a subframe");
		}
		return;
	}
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
	/* The document that sent the message: the app checks it on every message. */
	NSURL *senderURL = message.frameInfo.request.URL ?: message.webView.URL;
	const char *sender = senderURL.absoluteString ? [senderURL.absoluteString UTF8String] : "";
	goVitraMessage(windowID, (char *)[body UTF8String], (char *)sender);
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

- (void)windowDidResignKey:(NSNotification *)notification {
	if ([notification.object isKindOfClass:[VitraPanel class]]) {
		VitraPanel *panel = notification.object;
		if (panel.visible) {
			panel.hiddenAt = CFAbsoluteTimeGetCurrent();
			[panel orderOut:nil];
		}
	}
}

@end

struct VitraWin {
	NSWindow *window;
	WKWebView *view;
	VitraDropView *dropView;
	VitraWinDelegate *delegate;
	VitraMenuTarget *menuTarget;
	char *id;
	int maximized;
	int fullscreen;
	int above;
	int minimized;
	int hidden;
	int req_width;
	int req_height;
	char *icon_path;
	int panel;
};

static char *g_program_name = NULL;

void vitra_app_init(const char *prgname) {
	[NSApplication sharedApplication];
	[NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
	if (prgname != NULL && prgname[0] != '\0') {
		NSString *name = [NSString stringWithUTF8String:prgname];
		if (name != nil) {
			[[NSProcessInfo processInfo] setProcessName:name];
		}
		free(g_program_name);
		g_program_name = strdup(prgname);
	}
}

void vitra_app_set_accessory(int accessory) {
	[NSApp setActivationPolicy:accessory ? NSApplicationActivationPolicyAccessory : NSApplicationActivationPolicyRegular];
}

int vitra_app_is_accessory(void) {
	return NSApp.activationPolicy == NSApplicationActivationPolicyAccessory ? 1 : 0;
}

const char *vitra_get_program_name(void) {
	return g_program_name;
}

void vitra_app_run(void) {
	[NSApp run];
}

void vitra_app_quit(void) {
	vitra_clear_hotkeys();
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

int vitra_is_main_thread(void) {
	return [NSThread isMainThread] ? 1 : 0;
}

void vitra_idle_add(unsigned long long id) {
	dispatch_async(dispatch_get_main_queue(), ^{
		goVitraIdle(id);
	});
}

VitraWin *vitra_win_new(const char *id, const char *title, int width, int height, const char *uri, const char *preload, int panel) {
	VitraWin *w = (VitraWin *)calloc(1, sizeof(VitraWin));
	if (!w) {
		return NULL;
	}
	w->id = strdup(id ? id : "");
	w->req_width = width > 0 ? width : 1024;
	w->req_height = height > 0 ? height : 768;
	w->delegate = [[VitraWinDelegate alloc] initWithWindowID:w->id];
	w->menuTarget = [[VitraMenuTarget alloc] init];

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
	if (vitra_devtools) {
		if (@available(macOS 13.3, *)) {
			w->view.inspectable = YES;
		}
	}
	w->view.navigationDelegate = w->delegate;
	w->view.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;

	w->dropView = [[VitraDropView alloc] initWithFrame:frame windowID:w->id];
	[w->dropView addSubview:w->view];

	w->panel = panel ? 1 : 0;
	if (panel) {
		/* Non-activating: showing the panel does not pull the user's
		 * frontmost app back, as menu bar popovers behave. */
		VitraPanel *p = [[VitraPanel alloc] initWithContentRect:frame
							       styleMask:NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel
								 backing:NSBackingStoreBuffered
								   defer:NO];
		p.floatingPanel = YES;
		p.level = NSPopUpMenuWindowLevel;
		p.collectionBehavior = NSWindowCollectionBehaviorCanJoinAllSpaces | NSWindowCollectionBehaviorFullScreenAuxiliary;
		p.hidesOnDeactivate = NO;
		p.hasShadow = YES;
		p.opaque = NO;
		p.backgroundColor = [NSColor clearColor];
		w->dropView.wantsLayer = YES;
		w->dropView.layer.cornerRadius = 10.0;
		w->dropView.layer.masksToBounds = YES;
		w->window = p;
		w->hidden = 1;
	} else {
		NSUInteger style = NSWindowStyleMaskTitled | NSWindowStyleMaskClosable |
				   NSWindowStyleMaskMiniaturizable | NSWindowStyleMaskResizable;
		w->window = [[NSWindow alloc] initWithContentRect:frame
						    styleMask:style
						      backing:NSBackingStoreBuffered
							defer:NO];
	}
	/* vitra_win_free releases the window: closing it must not as well. */
	w->window.releasedWhenClosed = NO;
	w->window.title = title ? [NSString stringWithUTF8String:title] : @"";
	w->window.contentView = w->dropView;
	w->window.delegate = w->delegate;
	if (!panel) {
		[w->window center];
		[w->window makeKeyAndOrderFront:nil];
		[NSApp activateIgnoringOtherApps:YES];
	}

	if (uri && uri[0] != '\0') {
		NSURL *url = [NSURL URLWithString:[NSString stringWithUTF8String:uri]];
		if (url) {
			[w->view loadRequest:[NSURLRequest requestWithURL:url]];
		}
	}
	return w;
}

/* Gap between the tray icon and the panel, in points. */
static const CGFloat kVitraPanelGap = 4.0;

int vitra_panel_show(VitraWin *w, int x, int y, int aw, int ah, int has_anchor) {
	if (!w || !w->panel) {
		return 0;
	}
	NSArray<NSScreen *> *screens = [NSScreen screens];
	NSSize size = w->window.frame.size;
	NSScreen *screen = [NSScreen mainScreen];
	NSRect frame;
	if (has_anchor && screens.count > 0) {
		/* Top-left points → Cocoa coordinates (up from the primary bottom). */
		CGFloat top = NSMaxY(screens[0].frame);
		NSRect anchor = NSMakeRect(x, top - y - ah, aw, ah);
		NSPoint mid = NSMakePoint(NSMidX(anchor), NSMidY(anchor));
		for (NSScreen *s in screens) {
			if (NSPointInRect(mid, s.frame)) {
				screen = s;
				break;
			}
		}
		frame.size = size;
		frame.origin.x = NSMidX(anchor) - size.width / 2;
		if (NSMidY(anchor) < NSMidY(screen.frame)) {
			frame.origin.y = NSMaxY(anchor) + kVitraPanelGap; /* icon at the bottom: open upward */
		} else {
			frame.origin.y = NSMinY(anchor) - size.height - kVitraPanelGap;
		}
	} else {
		NSRect vis = screen.visibleFrame;
		frame = NSMakeRect(NSMidX(vis) - size.width / 2, NSMidY(vis) - size.height / 2, size.width, size.height);
	}
	NSRect vis = screen.visibleFrame;
	frame.origin.x = MAX(NSMinX(vis), MIN(frame.origin.x, NSMaxX(vis) - size.width));
	frame.origin.y = MAX(NSMinY(vis), MIN(frame.origin.y, NSMaxY(vis) - size.height));
	[w->window setFrame:frame display:YES];
	[w->window makeKeyAndOrderFront:nil];
	w->hidden = 0;
	return 1;
}

int vitra_panel_hide(VitraWin *w) {
	if (!w || !w->panel) {
		return 0;
	}
	[w->window orderOut:nil];
	w->hidden = 1;
	return 1;
}

int vitra_panel_shown(VitraWin *w) {
	if (!w || !w->panel) {
		return -1;
	}
	VitraPanel *p = (VitraPanel *)w->window;
	if (p.visible) {
		return 1;
	}
	return p.hiddenAt > 0 && CFAbsoluteTimeGetCurrent() - p.hiddenAt < kVitraPanelReopenGuard;
}

void vitra_win_frame(VitraWin *w, int *x, int *y, int *fw, int *fh) {
	NSArray<NSScreen *> *screens = [NSScreen screens];
	NSRect r = w->window.frame;
	CGFloat top = screens.count > 0 ? NSMaxY(screens[0].frame) : 0;
	*x = (int)NSMinX(r);
	*y = (int)(top - NSMaxY(r));
	*fw = (int)NSWidth(r);
	*fh = (int)NSHeight(r);
}

void vitra_panel_blur(VitraWin *w) {
	if (w && w->panel) {
		[w->delegate windowDidResignKey:[NSNotification notificationWithName:NSWindowDidResignKeyNotification object:w->window]];
	}
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
		[w->view removeFromSuperview];
		[w->view release];
		w->view = nil;
	}
	if (w->dropView) {
		[w->dropView release];
		w->dropView = nil;
	}
	if (w->window) {
		/* Autorelease: this may run inside the window's own close. */
		[w->window autorelease];
		w->window = nil;
	}
	if (w->delegate) {
		[w->delegate release];
		w->delegate = nil;
	}
	if (w->menuTarget) {
		[w->menuTarget release];
		w->menuTarget = nil;
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

void vitra_win_focus(VitraWin *w) {
	if (!w || !w->window) {
		return;
	}
	NSWindow *win = w->window;
	w->hidden = 0;
	w->minimized = 0;
	if (win.miniaturized) {
		[win deminiaturize:nil];
	}
	[win makeKeyAndOrderFront:nil];
	[NSApp activateIgnoringOtherApps:YES];
}

void vitra_win_blur(VitraWin *w) {
	if (!w || !w->window) {
		return;
	}
	NSWindow *win = w->window;
	[win resignKeyWindow];
	[win orderBack:nil];
}

static void parse_shortcut(const char *shortcut, NSString **keyOut, NSEventModifierFlags *modsOut) {
	*keyOut = @"";
	*modsOut = 0;
	if (!shortcut || shortcut[0] == '\0') {
		return;
	}
	NSString *raw = [NSString stringWithUTF8String:shortcut];
	NSArray *parts = [raw componentsSeparatedByString:@"+"];
	NSEventModifierFlags mods = 0;
	NSString *key = @"";
	for (NSString *part in parts) {
		NSString *p = [[part stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]] lowercaseString];
		if (p.length == 0) {
			continue;
		}
		if ([p isEqualToString:@"ctrl"] || [p isEqualToString:@"control"]) {
			/* Map Ctrl → Command for macOS menu conventions. */
			mods |= NSEventModifierFlagCommand;
		} else if ([p isEqualToString:@"cmd"] || [p isEqualToString:@"command"] || [p isEqualToString:@"meta"] || [p isEqualToString:@"super"]) {
			mods |= NSEventModifierFlagCommand;
		} else if ([p isEqualToString:@"shift"]) {
			mods |= NSEventModifierFlagShift;
		} else if ([p isEqualToString:@"alt"] || [p isEqualToString:@"option"]) {
			mods |= NSEventModifierFlagOption;
		} else if (p.length >= 1) {
			key = [p substringToIndex:1];
		}
	}
	*keyOut = key;
	*modsOut = mods;
}

static NSMenuItem *find_or_create_top(NSMenu *main, const char *menu_label) {
	NSString *title = [NSString stringWithUTF8String:menu_label ? menu_label : ""];
	for (NSMenuItem *item in main.itemArray) {
		if ([item.title isEqualToString:title]) {
			if (!item.submenu) {
				NSMenu *sub = [[NSMenu alloc] initWithTitle:title];
				item.submenu = sub;
				[sub release];
			}
			return item;
		}
	}
	NSMenuItem *top = [[NSMenuItem alloc] initWithTitle:title action:NULL keyEquivalent:@""];
	NSMenu *sub = [[NSMenu alloc] initWithTitle:title];
	top.submenu = sub;
	[sub release];
	[main addItem:top];
	[top release];
	return top;
}

void vitra_win_clear_menu(VitraWin *w) {
	(void)w;
	NSMenu *main = [[NSMenu alloc] initWithTitle:@"MainMenu"];
	[NSApp setMainMenu:main];
	[main release];
}

/* new_menu_item builds an action item (or a separator) with flags applied.
 * The menu holding it must have autoenablesItems off for Disabled to stick. */
static NSMenuItem *new_menu_item(id target, const char *item_id, const char *item_label, NSString *key, int flags) {
	if (flags & VITRA_MENU_SEPARATOR) {
		return [[NSMenuItem separatorItem] retain];
	}
	NSMenuItem *item = [[NSMenuItem alloc]
	    initWithTitle:[NSString stringWithUTF8String:item_label]
		   action:@selector(onAction:)
	    keyEquivalent:key];
	item.target = target;
	item.representedObject = [NSString stringWithUTF8String:item_id];
	item.enabled = (flags & VITRA_MENU_DISABLED) ? NO : YES;
	item.state = (flags & VITRA_MENU_CHECKED) ? NSControlStateValueOn : NSControlStateValueOff;
	return item;
}

void vitra_win_add_menu_item(VitraWin *w, const char *menu_label, const char *item_id, const char *item_label, const char *shortcut, int flags) {
	int separator = (flags & VITRA_MENU_SEPARATOR) != 0;
	if (!w || !menu_label || (!separator && (!item_id || !item_label))) {
		return;
	}
	NSMenu *main = [NSApp mainMenu];
	if (!main) {
		main = [[NSMenu alloc] initWithTitle:@"MainMenu"];
		[NSApp setMainMenu:main];
		[main release];
		main = [NSApp mainMenu];
	}
	NSMenuItem *top = find_or_create_top(main, menu_label);
	top.submenu.autoenablesItems = NO;
	NSString *key = @"";
	NSEventModifierFlags mods = 0;
	parse_shortcut(shortcut, &key, &mods);
	NSMenuItem *item = new_menu_item(w->menuTarget, item_id, item_label, key, flags);
	if (!separator) {
		item.keyEquivalentModifierMask = mods;
	}
	[top.submenu addItem:item];
	[item release];
}

static int shortcut_matches(NSMenuItem *item, NSString *key, NSEventModifierFlags mods) {
	if (!item || key.length == 0) {
		return 0;
	}
	if (![item.keyEquivalent.lowercaseString isEqualToString:key.lowercaseString]) {
		return 0;
	}
	/* Ignore device-dependent bits; compare standard modifier flags. */
	NSEventModifierFlags mask = NSEventModifierFlagCommand | NSEventModifierFlagShift | NSEventModifierFlagOption | NSEventModifierFlagControl;
	return (item.keyEquivalentModifierMask & mask) == (mods & mask);
}

static int activate_in_menu(NSMenu *menu, NSString *key, NSEventModifierFlags mods) {
	if (!menu) {
		return 0;
	}
	for (NSMenuItem *item in menu.itemArray) {
		if (item.hasSubmenu) {
			if (activate_in_menu(item.submenu, key, mods)) {
				return 1;
			}
			continue;
		}
		if (shortcut_matches(item, key, mods) && item.target && item.action) {
			[item.target performSelector:item.action withObject:item];
			return 1;
		}
	}
	return 0;
}

int vitra_win_activate_accel(VitraWin *w, const char *shortcut) {
	(void)w;
	NSString *key = @"";
	NSEventModifierFlags mods = 0;
	parse_shortcut(shortcut, &key, &mods);
	if (key.length == 0) {
		return 0;
	}
	return activate_in_menu([NSApp mainMenu], key, mods);
}

static NSStatusItem *g_tray = nil;
static VitraMenuTarget *g_tray_target = nil;
/* g_tray_menu is the tray menu. It is the status item's menu (opened on any
 * click) unless g_tray_click_activates, when a right click opens it. */
static NSMenu *g_tray_menu = nil;
static int g_tray_click_activates = 0;

/* Menu bar icons are 18pt tall; wider images keep their aspect ratio. */
static const CGFloat kVitraTrayIconHeight = 18.0;

static NSImage *tray_image(const void *icon, int icon_len, int template_icon) {
	if (!icon || icon_len <= 0) {
		return nil;
	}
	NSData *data = [NSData dataWithBytes:icon length:(NSUInteger)icon_len];
	NSImage *img = [[[NSImage alloc] initWithData:data] autorelease];
	if (!img || img.size.height <= 0) {
		return nil;
	}
	CGFloat scale = kVitraTrayIconHeight / img.size.height;
	img.size = NSMakeSize(img.size.width * scale, kVitraTrayIconHeight);
	img.template = template_icon ? YES : NO;
	return img;
}

static void vitra_tray_show_menu(void) {
	if (!g_tray || !g_tray_menu) {
		return;
	}
	/* Attach the menu for one click: performClick tracks it modally. */
	g_tray.menu = g_tray_menu;
	[g_tray.button performClick:nil];
	g_tray.menu = nil;
}

void vitra_tray_set(const char *tooltip, const char *title, const void *icon, int icon_len, int template_icon, int click_activates) {
	if (!g_tray_target) {
		g_tray_target = [[VitraMenuTarget alloc] init];
	}
	if (!g_tray) {
		g_tray = [[[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength] retain];
	}
	NSImage *img = tray_image(icon, icon_len, template_icon);
	NSString *text = title ? [NSString stringWithUTF8String:title] : @"";
	if (!img && text.length == 0) {
		/* Neither icon nor title: show the app name so the item is visible. */
		text = [[NSProcessInfo processInfo] processName];
	}
	g_tray.button.image = img;
	g_tray.button.imagePosition = text.length > 0 ? NSImageLeading : NSImageOnly;
	g_tray.button.title = text;
	g_tray.button.toolTip = tooltip ? [NSString stringWithUTF8String:tooltip] : @"";
	g_tray_click_activates = click_activates;
	if (click_activates) {
		g_tray.menu = nil;
		g_tray.button.target = g_tray_target;
		g_tray.button.action = @selector(onTrayClick:);
		[g_tray.button sendActionOn:NSEventMaskLeftMouseUp | NSEventMaskRightMouseUp];
	} else {
		g_tray.button.target = nil;
		g_tray.button.action = NULL;
		g_tray.menu = g_tray_menu;
	}
	g_tray.visible = YES;
}

/* vitra_rect_placed reports whether a rectangle in Cocoa screen
 * coordinates has its middle on a screen. Not "fits inside one": on
 * macOS 27 the status item's window rises a point above the screen's top
 * edge, and requiring containment read every click as not placed yet. */
int vitra_rect_placed(double x, double y, double w, double h) {
	if (w <= 0 || h <= 0) {
		return 0;
	}
	NSPoint mid = NSMakePoint(x + w / 2, y + h / 2);
	for (NSScreen *screen in [NSScreen screens]) {
		if (NSPointInRect(mid, screen.frame)) {
			return 1;
		}
	}
	return 0;
}

int vitra_tray_anchor(int *x, int *y, int *w, int *h) {
	NSWindow *win = g_tray ? g_tray.button.window : nil;
	NSArray<NSScreen *> *screens = [NSScreen screens];
	if (!win || screens.count == 0) {
		return 0;
	}
	NSRect r = win.frame;
	/* Until the menu bar places it, the item sits off screen. */
	if (!vitra_rect_placed(NSMinX(r), NSMinY(r), NSWidth(r), NSHeight(r))) {
		return 0;
	}
	/* Cocoa screens grow upward from the primary screen's bottom-left. */
	CGFloat top = NSMaxY(screens[0].frame);
	*x = (int)NSMinX(r);
	*y = (int)(top - NSMaxY(r));
	*w = (int)NSWidth(r);
	*h = (int)NSHeight(r);
	return 1;
}

void vitra_tray_click(void) {
	if (g_tray && g_tray_click_activates) {
		[g_tray_target onTrayClick:g_tray.button];
	}
}

void vitra_tray_clear_menu(void) {
	if (g_tray) {
		g_tray.menu = nil;
	}
	[g_tray_menu release];
	g_tray_menu = nil;
}

void vitra_tray_add_menu_item(const char *item_id, const char *item_label, int flags) {
	int separator = (flags & VITRA_MENU_SEPARATOR) != 0;
	if (!separator && (!item_id || !item_label)) {
		return;
	}
	if (!g_tray) {
		vitra_tray_set("", "", NULL, 0, 0, 0);
	}
	if (!g_tray_target) {
		g_tray_target = [[VitraMenuTarget alloc] init];
	}
	if (!g_tray_menu) {
		g_tray_menu = [[NSMenu alloc] initWithTitle:@"Tray"];
		g_tray_menu.autoenablesItems = NO;
	}
	if (!g_tray_click_activates) {
		g_tray.menu = g_tray_menu;
	}
	NSMenuItem *item = new_menu_item(g_tray_target, item_id, item_label, @"", flags);
	[g_tray_menu addItem:item];
	[item release];
}

char *vitra_tray_state(void) {
	if (!g_tray) {
		return strdup("");
	}
	NSImage *img = g_tray.button.image;
	NSMutableString *out = [NSMutableString stringWithFormat:@"%@\n%d\n%d\n",
		g_tray.button.title ?: @"", img != nil, img != nil && img.template];
	for (NSMenuItem *item in g_tray_menu.itemArray) {
		if (item.separatorItem) {
			[out appendString:@"-\n"];
			continue;
		}
		int flags = (item.enabled ? 0 : VITRA_MENU_DISABLED) | (item.state == NSControlStateValueOn ? VITRA_MENU_CHECKED : 0);
		[out appendFormat:@"%@\t%d\n", item.title, flags];
	}
	return strdup(out.UTF8String);
}

void vitra_tray_clear(void) {
	vitra_tray_clear_menu();
	if (g_tray) {
		[[NSStatusBar systemStatusBar] removeStatusItem:g_tray];
		[g_tray release];
		g_tray = nil;
	}
}

void vitra_win_set_drag_drop(VitraWin *w, int enabled) {
	if (!w || !w->dropView) {
		return;
	}
	[w->dropView setDropEnabled:enabled];
}


/* vitra_apply_filters restricts panel to the extensions in filters, which is
 * "Label:ext1,ext2;Other:ext3". */
static void vitra_apply_filters(NSSavePanel *panel, const char *filters) {
	if (!filters || !filters[0]) {
		return;
	}
	NSMutableArray<UTType *> *types = [NSMutableArray array];
	NSString *spec = [NSString stringWithUTF8String:filters];
	for (NSString *group in [spec componentsSeparatedByString:@";"]) {
		NSRange colon = [group rangeOfString:@":"];
		if (colon.location == NSNotFound) {
			continue;
		}
		NSString *list = [group substringFromIndex:colon.location + 1];
		for (NSString *ext in [list componentsSeparatedByString:@","]) {
			NSString *trimmed = [ext stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
			UTType *type = trimmed.length > 0 ? [UTType typeWithFilenameExtension:trimmed] : nil;
			if (type) {
				[types addObject:type];
			}
		}
	}
	if (types.count > 0) {
		panel.allowedContentTypes = types;
	}
}

char *vitra_open_dialog(const char *title, const char *default_path, const char *filters) {
	NSOpenPanel *panel = [NSOpenPanel openPanel];
	panel.canChooseFiles = YES;
	panel.canChooseDirectories = NO;
	panel.allowsMultipleSelection = NO;
	panel.resolvesAliases = YES;
	if (title && title[0]) {
		panel.title = [NSString stringWithUTF8String:title];
		panel.message = panel.title;
	}
	if (default_path && default_path[0]) {
		panel.directoryURL = [NSURL fileURLWithPath:[NSString stringWithUTF8String:default_path] isDirectory:YES];
	}
	vitra_apply_filters(panel, filters);
	if ([panel runModal] != NSModalResponseOK) {
		return NULL;
	}
	NSURL *url = panel.URL;
	if (!url || !url.path) {
		return NULL;
	}
	return strdup(url.fileSystemRepresentation);
}

/* vitra_open_dialog_multi runs a multi-select NSOpenPanel. It returns the
 * selected paths as one malloc'd buffer of NUL-terminated paths (NUL is the
 * only byte a path cannot contain), its byte length in *out_len, or NULL when
 * the user cancels or picks nothing. Free it with free(). The panel and its
 * URLs are autoreleased; nothing here is retained. */
char *vitra_open_dialog_multi(const char *title, const char *default_path, const char *filters, int *out_len) {
	*out_len = 0;
	NSOpenPanel *panel = [NSOpenPanel openPanel];
	panel.canChooseFiles = YES;
	panel.canChooseDirectories = NO;
	panel.allowsMultipleSelection = YES;
	panel.resolvesAliases = YES;
	if (title && title[0]) {
		panel.title = [NSString stringWithUTF8String:title];
		panel.message = panel.title;
	}
	if (default_path && default_path[0]) {
		panel.directoryURL = [NSURL fileURLWithPath:[NSString stringWithUTF8String:default_path] isDirectory:YES];
	}
	vitra_apply_filters(panel, filters);
	if ([panel runModal] != NSModalResponseOK) {
		return NULL;
	}
	size_t total = 0;
	for (NSURL *url in panel.URLs) {
		if (url.isFileURL) {
			total += strlen(url.fileSystemRepresentation) + 1;
		}
	}
	if (total == 0 || total > INT_MAX) {
		return NULL;
	}
	char *buf = malloc(total);
	if (!buf) {
		return NULL;
	}
	size_t off = 0;
	for (NSURL *url in panel.URLs) {
		if (!url.isFileURL) {
			continue;
		}
		const char *p = url.fileSystemRepresentation;
		size_t n = strlen(p) + 1;
		memcpy(buf + off, p, n);
		off += n;
	}
	*out_len = (int)off;
	return buf;
}

char *vitra_open_directory_dialog(const char *title, const char *default_path) {
	NSOpenPanel *panel = [NSOpenPanel openPanel];
	panel.canChooseFiles = NO;
	panel.canChooseDirectories = YES;
	panel.allowsMultipleSelection = NO;
	panel.resolvesAliases = YES;
	panel.canCreateDirectories = YES;
	if (title && title[0]) {
		panel.title = [NSString stringWithUTF8String:title];
		panel.message = panel.title;
	}
	if (default_path && default_path[0]) {
		panel.directoryURL = [NSURL fileURLWithPath:[NSString stringWithUTF8String:default_path] isDirectory:YES];
	}
	if ([panel runModal] != NSModalResponseOK) {
		return NULL;
	}
	NSURL *url = panel.URL;
	if (!url || !url.path) {
		return NULL;
	}
	return strdup(url.fileSystemRepresentation);
}

char *vitra_save_dialog(const char *title, const char *default_path, const char *filters) {
	NSSavePanel *panel = [NSSavePanel savePanel];
	panel.canCreateDirectories = YES;
	if (title && title[0]) {
		panel.title = [NSString stringWithUTF8String:title];
		panel.message = panel.title;
	}
	if (default_path && default_path[0]) {
		NSString *path = [NSString stringWithUTF8String:default_path];
		BOOL isDir = NO;
		[[NSFileManager defaultManager] fileExistsAtPath:path isDirectory:&isDir];
		if (isDir) {
			panel.directoryURL = [NSURL fileURLWithPath:path isDirectory:YES];
		} else {
			panel.directoryURL = [NSURL fileURLWithPath:[path stringByDeletingLastPathComponent] isDirectory:YES];
			panel.nameFieldStringValue = [path lastPathComponent];
		}
	}
	vitra_apply_filters(panel, filters);
	if ([panel runModal] != NSModalResponseOK) {
		return NULL;
	}
	NSURL *url = panel.URL;
	if (!url || !url.path) {
		return NULL;
	}
	return strdup(url.fileSystemRepresentation);
}

int vitra_message_dialog(const char *title, const char *message, int confirm) {
	@autoreleasepool {
		NSAlert *alert = [[NSAlert alloc] init];
		alert.messageText = title ? [NSString stringWithUTF8String:title] : @"";
		alert.informativeText = message ? [NSString stringWithUTF8String:message] : @"";
		if (confirm) {
			alert.alertStyle = NSAlertStyleInformational;
			[alert addButtonWithTitle:@"Yes"];
			[alert addButtonWithTitle:@"No"];
		} else {
			alert.alertStyle = NSAlertStyleInformational;
			[alert addButtonWithTitle:@"OK"];
		}
		NSModalResponse response = [alert runModal];
		if (confirm) {
			return response == NSAlertFirstButtonReturn ? 1 : 0;
		}
		return 1;
	}
}

int vitra_has_bundle_id(void) {
	@autoreleasepool {
		return [[NSBundle mainBundle] bundleIdentifier].length > 0 ? 1 : 0;
	}
}

/* Requires a bundle identifier (UNUserNotificationCenter raises without
 * one); callers check vitra_has_bundle_id first. Delivery is asynchronous
 * and needs the user's permission, which macOS asks for once. */
int vitra_show_notification(const char *title, const char *body) {
	@autoreleasepool {
		if (!vitra_has_bundle_id()) {
			return 0;
		}
		UNMutableNotificationContent *content = [[[UNMutableNotificationContent alloc] init] autorelease];
		content.title = title ? [NSString stringWithUTF8String:title] : @"";
		content.body = body ? [NSString stringWithUTF8String:body] : @"";
		UNNotificationRequest *req = [UNNotificationRequest requestWithIdentifier:[[NSUUID UUID] UUIDString]
									     content:content
									     trigger:nil];
		UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
		[center requestAuthorizationWithOptions:UNAuthorizationOptionAlert
				      completionHandler:^(BOOL granted, NSError *error) {
					      (void)error;
					      if (granted) {
						      [center addNotificationRequest:req withCompletionHandler:nil];
					      }
				      }];
		return 1;
	}
}

#define VITRA_MAX_HOTKEYS 64
#define VITRA_HOTKEY_SIG 'VTRA'

typedef struct {
	EventHotKeyRef ref;
	UInt32 id;
	char *accel;
	char *action;
} HotkeyEntry;

static HotkeyEntry g_hotkeys[VITRA_MAX_HOTKEYS];
static int g_hotkey_n = 0;
static UInt32 g_hotkey_next = 1;
static EventHandlerRef g_hotkey_handler = NULL;

static int parse_hotkey(const char *shortcut, UInt32 *mods, UInt32 *vk) {
	if (!shortcut || !mods || !vk) {
		return 0;
	}
	*mods = 0;
	*vk = 0;
	char buf[128];
	strncpy(buf, shortcut, sizeof(buf) - 1);
	buf[sizeof(buf) - 1] = '\0';
	for (char *p = buf; *p; p++) {
		if (*p == '<' || *p == '>') {
			*p = '+';
		}
	}
	char key = 0;
	char *cursor = buf;
	while (*cursor) {
		while (*cursor == '+') {
			cursor++;
		}
		if (*cursor == '\0') {
			break;
		}
		char *tok = cursor;
		while (*cursor && *cursor != '+') {
			cursor++;
		}
		if (*cursor == '+') {
			*cursor++ = '\0';
		}
		while (*tok && isspace((unsigned char)*tok)) {
			tok++;
		}
		char *end = tok + strlen(tok);
		while (end > tok && isspace((unsigned char)end[-1])) {
			*--end = '\0';
		}
		char lower[64];
		size_t n = strlen(tok);
		if (n >= sizeof(lower)) {
			n = sizeof(lower) - 1;
		}
		for (size_t i = 0; i < n; i++) {
			lower[i] = (char)tolower((unsigned char)tok[i]);
		}
		lower[n] = '\0';
		if (strcmp(lower, "ctrl") == 0 || strcmp(lower, "control") == 0 ||
			strcmp(lower, "cmd") == 0 || strcmp(lower, "command") == 0 ||
			strcmp(lower, "meta") == 0 || strcmp(lower, "super") == 0 ||
			strcmp(lower, "win") == 0) {
			/* Ctrl maps to Command for macOS conventions (matches menus). */
			*mods |= cmdKey;
		} else if (strcmp(lower, "shift") == 0) {
			*mods |= shiftKey;
		} else if (strcmp(lower, "alt") == 0 || strcmp(lower, "option") == 0) {
			*mods |= optionKey;
		} else if (n == 1) {
			key = (char)toupper((unsigned char)lower[0]);
		}
	}
	if (key == 0 || *mods == 0) {
		return 0;
	}
	if (key < 'A' || key > 'Z') {
		return 0;
	}
	/* kVK_ANSI_A is 0x00, B is 0x0B, … — use the standard letter table. */
	static const UInt32 letterVK[26] = {
		kVK_ANSI_A, kVK_ANSI_B, kVK_ANSI_C, kVK_ANSI_D, kVK_ANSI_E, kVK_ANSI_F,
		kVK_ANSI_G, kVK_ANSI_H, kVK_ANSI_I, kVK_ANSI_J, kVK_ANSI_K, kVK_ANSI_L,
		kVK_ANSI_M, kVK_ANSI_N, kVK_ANSI_O, kVK_ANSI_P, kVK_ANSI_Q, kVK_ANSI_R,
		kVK_ANSI_S, kVK_ANSI_T, kVK_ANSI_U, kVK_ANSI_V, kVK_ANSI_W, kVK_ANSI_X,
		kVK_ANSI_Y, kVK_ANSI_Z,
	};
	*vk = letterVK[key - 'A'];
	return 1;
}

static OSStatus on_hotkey(EventHandlerCallRef next, EventRef event, void *data) {
	(void)next;
	(void)data;
	EventHotKeyID hk;
	if (GetEventParameter(event, kEventParamDirectObject, typeEventHotKeyID, NULL, sizeof(hk), NULL, &hk) != noErr) {
		return noErr;
	}
	for (int i = 0; i < g_hotkey_n; i++) {
		if (g_hotkeys[i].id == hk.id && g_hotkeys[i].action) {
			goVitraAction(g_hotkeys[i].action);
			break;
		}
	}
	return noErr;
}

static void ensure_hotkey_handler(void) {
	if (g_hotkey_handler) {
		return;
	}
	EventTypeSpec spec = {kEventClassKeyboard, kEventHotKeyPressed};
	InstallApplicationEventHandler(NewEventHandlerUPP(on_hotkey), 1, &spec, NULL, &g_hotkey_handler);
}

static void free_hotkey_at(int i) {
	if (g_hotkeys[i].ref) {
		UnregisterEventHotKey(g_hotkeys[i].ref);
		g_hotkeys[i].ref = NULL;
	}
	free(g_hotkeys[i].accel);
	free(g_hotkeys[i].action);
	g_hotkeys[i].accel = NULL;
	g_hotkeys[i].action = NULL;
	g_hotkeys[i].id = 0;
}

int vitra_register_hotkey(const char *accelerator, const char *action_id) {
	if (!accelerator || !action_id || accelerator[0] == '\0' || action_id[0] == '\0') {
		return 0;
	}
	UInt32 mods = 0, vk = 0;
	if (!parse_hotkey(accelerator, &mods, &vk)) {
		return 0;
	}
	ensure_hotkey_handler();
	for (int i = 0; i < g_hotkey_n; i++) {
		if (g_hotkeys[i].accel && strcmp(g_hotkeys[i].accel, accelerator) == 0) {
			if (g_hotkeys[i].ref) {
				UnregisterEventHotKey(g_hotkeys[i].ref);
				g_hotkeys[i].ref = NULL;
			}
			free(g_hotkeys[i].action);
			g_hotkeys[i].action = strdup(action_id);
			EventHotKeyID hk = {.signature = VITRA_HOTKEY_SIG, .id = g_hotkeys[i].id};
			if (RegisterEventHotKey(vk, mods, hk, GetApplicationEventTarget(), 0, &g_hotkeys[i].ref) != noErr) {
				return 0;
			}
			return 1;
		}
	}
	if (g_hotkey_n >= VITRA_MAX_HOTKEYS) {
		return 0;
	}
	UInt32 id = g_hotkey_next++;
	EventHotKeyID hk = {.signature = VITRA_HOTKEY_SIG, .id = id};
	EventHotKeyRef ref = NULL;
	if (RegisterEventHotKey(vk, mods, hk, GetApplicationEventTarget(), 0, &ref) != noErr) {
		return 0;
	}
	g_hotkeys[g_hotkey_n].ref = ref;
	g_hotkeys[g_hotkey_n].id = id;
	g_hotkeys[g_hotkey_n].accel = strdup(accelerator);
	g_hotkeys[g_hotkey_n].action = strdup(action_id);
	g_hotkey_n++;
	return 1;
}

int vitra_unregister_hotkey(const char *accelerator) {
	if (!accelerator) {
		return 0;
	}
	for (int i = 0; i < g_hotkey_n; i++) {
		if (g_hotkeys[i].accel && strcmp(g_hotkeys[i].accel, accelerator) == 0) {
			free_hotkey_at(i);
			g_hotkeys[i] = g_hotkeys[g_hotkey_n - 1];
			memset(&g_hotkeys[g_hotkey_n - 1], 0, sizeof(g_hotkeys[0]));
			g_hotkey_n--;
			return 1;
		}
	}
	return 0;
}

void vitra_clear_hotkeys(void) {
	for (int i = 0; i < g_hotkey_n; i++) {
		free_hotkey_at(i);
	}
	g_hotkey_n = 0;
	g_hotkey_next = 1;
}

int vitra_login_item_status(void) {
	if (@available(macOS 13.0, *)) {
		return (int)SMAppService.mainAppService.status;
	}
	return -1;
}

char *vitra_login_item_set(int enabled) {
	if (@available(macOS 13.0, *)) {
		NSError *err = nil;
		SMAppService *service = SMAppService.mainAppService;
		BOOL ok = enabled ? [service registerAndReturnError:&err] : [service unregisterAndReturnError:&err];
		if (!ok) {
			/* Unregistering an item that is not registered is not an error. */
			if (!enabled && service.status == SMAppServiceStatusNotRegistered) {
				return NULL;
			}
			NSString *msg = err.localizedDescription ?: @"SMAppService failed";
			return strdup(msg.UTF8String);
		}
		return NULL;
	}
	return strdup("login items need macOS 13 or later");
}
