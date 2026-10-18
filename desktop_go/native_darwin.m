//go:build darwin

#import <Cocoa/Cocoa.h>
#include <string.h>

void music_dl_install_edit_menu(void) {
    NSMenu *bar = [NSApp mainMenu];
    if (!bar) {
        bar = [[NSMenu alloc] initWithTitle:@""];
        [NSApp setMainMenu:bar];
        NSMenuItem *appItem = [[NSMenuItem alloc] initWithTitle:@"" action:nil keyEquivalent:@""];
        NSMenu *appMenu = [[NSMenu alloc] initWithTitle:@"music-dl"];
        [appMenu addItemWithTitle:@"退出 music-dl" action:@selector(terminate:) keyEquivalent:@"q"];
        [appItem setSubmenu:appMenu];
        [bar addItem:appItem];
    }
    NSMenu *edit = [[NSMenu alloc] initWithTitle:@"编辑"];
    [edit addItemWithTitle:@"撤销" action:@selector(undo:) keyEquivalent:@"z"];
    NSMenuItem *redo = [edit addItemWithTitle:@"重做" action:@selector(redo:) keyEquivalent:@"z"];
    [redo setKeyEquivalentModifierMask:NSEventModifierFlagCommand | NSEventModifierFlagShift];
    [edit addItem:[NSMenuItem separatorItem]];
    [edit addItemWithTitle:@"剪切" action:@selector(cut:) keyEquivalent:@"x"];
    [edit addItemWithTitle:@"复制" action:@selector(copy:) keyEquivalent:@"c"];
    [edit addItemWithTitle:@"粘贴" action:@selector(paste:) keyEquivalent:@"v"];
    [edit addItemWithTitle:@"全选" action:@selector(selectAll:) keyEquivalent:@"a"];
    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:@"编辑" action:nil keyEquivalent:@""];
    [item setSubmenu:edit];
    [bar addItem:item];
}

char *music_dl_pick_folder(void) {
    @autoreleasepool {
        NSOpenPanel *panel = [NSOpenPanel openPanel];
        [panel setTitle:@"选择下载目录"];
        [panel setPrompt:@"选择目录"];
        [panel setCanChooseFiles:NO];
        [panel setCanChooseDirectories:YES];
        [panel setAllowsMultipleSelection:NO];
        [panel setCanCreateDirectories:YES];
        if ([panel runModal] != NSModalResponseOK) return NULL;
        return strdup([[[panel URL] path] UTF8String]);
    }
}
