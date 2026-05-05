#import <Cocoa/Cocoa.h>
#include "_cgo_export.h"
#include "handler_darwin.h"

@interface BPDelegate : NSObject <NSApplicationDelegate>
@end

@implementation BPDelegate

- (void)applicationWillFinishLaunching:(NSNotification *)note {
    [[NSAppleEventManager sharedAppleEventManager]
        setEventHandler:self
            andSelector:@selector(getUrl:withReplyEvent:)
          forEventClass:kInternetEventClass
             andEventID:kAEGetURL];
}

- (void)getUrl:(NSAppleEventDescriptor *)event withReplyEvent:(NSAppleEventDescriptor *)reply {
    NSString *url = [[event paramDescriptorForKeyword:keyDirectObject] stringValue];
    if (!url) return;

    // Capture the source app *now*, while the originating app is still frontmost
    // (we have LSUIElement=true, so we don't steal focus).
    NSRunningApplication *front = [[NSWorkspace sharedWorkspace] frontmostApplication];
    NSString *sourceName = (front && front.localizedName)    ? front.localizedName    : @"";
    NSString *sourceID   = (front && front.bundleIdentifier) ? front.bundleIdentifier : @"";

    HandleURL((char *)[url UTF8String],
              (char *)[sourceID UTF8String],
              (char *)[sourceName UTF8String]);
}

@end

void RunMacApp(void) {
    @autoreleasepool {
        NSApplication *app = [NSApplication sharedApplication];
        BPDelegate *delegate = [[BPDelegate alloc] init];
        [app setDelegate:delegate];
        [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
        [app run];
    }
}
