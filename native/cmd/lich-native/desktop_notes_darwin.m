// UNUserNotificationCenter for desktop_notes_darwin.go. The delegate hands a
// click back to Go as (session id, action): "open", "dismiss" or "gone".

#import <Foundation/Foundation.h>
#import <UserNotifications/UserNotifications.h>
#include <stdbool.h>

#include "_cgo_export.h"

static NSString *const lichCategory = @"lich.session";

@interface LichNotesDelegate : NSObject <UNUserNotificationCenterDelegate>
@end

@implementation LichNotesDelegate

// Without this a note raised while lich is the frontmost app is dropped; the
// Go side already decided the session needs telling.
- (void)userNotificationCenter:(UNUserNotificationCenter *)center
       willPresentNotification:(UNNotification *)notification
         withCompletionHandler:(void (^)(UNNotificationPresentationOptions))completionHandler {
	completionHandler(UNNotificationPresentationOptionBanner | UNNotificationPresentationOptionList |
	                  UNNotificationPresentationOptionSound);
}

- (void)userNotificationCenter:(UNUserNotificationCenter *)center
    didReceiveNotificationResponse:(UNNotificationResponse *)response
             withCompletionHandler:(void (^)(void))completionHandler {
	NSString *action = response.actionIdentifier;
	const char *key = "gone";
	if ([action isEqualToString:UNNotificationDefaultActionIdentifier] || [action isEqualToString:@"open"]) {
		key = "open";
	} else if ([action isEqualToString:@"dismiss"]) {
		key = "dismiss";
	}
	lichNoteAnswered((char *)response.notification.request.identifier.UTF8String, (char *)key);
	completionHandler();
}

@end

static LichNotesDelegate *lichDelegate;

bool lichNotesStart(const char *open, const char *dismiss) {
	// The center raises an exception outside a bundle (a bare lich-native, or
	// `go run`), which would take the window down with it.
	if (NSBundle.mainBundle.bundleIdentifier == nil) {
		return false;
	}
	UNUserNotificationCenter *center = UNUserNotificationCenter.currentNotificationCenter;
	lichDelegate = [LichNotesDelegate new];
	center.delegate = lichDelegate;
	UNNotificationAction *openAction =
	    [UNNotificationAction actionWithIdentifier:@"open"
	                                         title:[NSString stringWithUTF8String:open]
	                                       options:UNNotificationActionOptionForeground];
	UNNotificationAction *dismissAction = [UNNotificationAction actionWithIdentifier:@"dismiss"
	                                                                           title:[NSString stringWithUTF8String:dismiss]
	                                                                         options:UNNotificationActionOptionNone];
	UNNotificationCategory *category =
	    [UNNotificationCategory categoryWithIdentifier:lichCategory
	                                           actions:@[ openAction, dismissAction ]
	                                 intentIdentifiers:@[]
	                                           options:UNNotificationCategoryOptionCustomDismissAction];
	[center setNotificationCategories:[NSSet setWithObject:category]];
	[center requestAuthorizationWithOptions:(UNAuthorizationOptionAlert | UNAuthorizationOptionSound)
	                      completionHandler:^(BOOL granted, NSError *error) {
		                    if (!granted) {
			                    NSLog(@"lich: notifications not allowed: %@", error);
		                    }
	                      }];
	return true;
}

void lichNotesShow(const char *id, const char *title, const char *body) {
	UNMutableNotificationContent *content = [UNMutableNotificationContent new];
	content.title = [NSString stringWithUTF8String:title];
	content.body = [NSString stringWithUTF8String:body];
	content.categoryIdentifier = lichCategory;
	content.sound = UNNotificationSound.defaultSound;
	UNNotificationRequest *request = [UNNotificationRequest requestWithIdentifier:[NSString stringWithUTF8String:id]
	                                                                      content:content
	                                                                      trigger:nil];
	[UNUserNotificationCenter.currentNotificationCenter addNotificationRequest:request
	                                                     withCompletionHandler:^(NSError *error) {
		                                                     if (error != nil) {
			                                                     NSLog(@"lich: notify: %@", error);
		                                                     }
	                                                     }];
}

void lichNotesWithdraw(const char *id) {
	NSArray<NSString *> *ids = @[ [NSString stringWithUTF8String:id] ];
	UNUserNotificationCenter *center = UNUserNotificationCenter.currentNotificationCenter;
	[center removeDeliveredNotificationsWithIdentifiers:ids];
	[center removePendingNotificationRequestsWithIdentifiers:ids];
}
