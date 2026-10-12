#import <Cocoa/Cocoa.h>
#include <stdlib.h>
#include "quiet_darwin.h"

// The app that was in front when this process started (OD2_QUIET only): the owner's current window. The window
// toolkit activates the new game for a moment while it opens; quietApplyNow hands the focus back to this app.
static NSRunningApplication *quietPrev = nil;

__attribute__((constructor)) static void quietRemember(void) {
	const char *v = getenv("OD2_QUIET");
	if (v == NULL || v[0] == 0 || (v[0] == '0' && v[1] == 0)) {
		return;
	}
	@autoreleasepool {
		NSRunningApplication *f = [[NSWorkspace sharedWorkspace] frontmostApplication];
		if (f != nil && f.processIdentifier != getpid()) {
			quietPrev = [f retain]; // not compiled with ARC: keep it past the autorelease pool
		}
	}
}

// quietApplyNow runs on the main thread: no Dock icon and never the active app (accessory policy), every window
// sent behind all others and deaf to the real mouse, the focus returned to the app the owner was using. The Metal
// layer keeps drawing, because the window stays on screen, just not in front.
static void quietApplyNow(void) {
	[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
	for (NSWindow *w in [NSApp windows]) {
		[w setIgnoresMouseEvents:YES];
		[w setCanHide:NO];
		[w setHidesOnDeactivate:NO];
		[w setLevel:NSNormalWindowLevel];
		[w orderBack:nil];
	}
	if ([NSApp isActive] && quietPrev != nil && !quietPrev.terminated) {
		[quietPrev activateWithOptions:0];
	}
}

void quietApply(void) {
	dispatch_async(dispatch_get_main_queue(), ^{ quietApplyNow(); });
}
