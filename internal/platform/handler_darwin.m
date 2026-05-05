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
    if (url) {
        HandleURL((char *)[url UTF8String]);
    }
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
