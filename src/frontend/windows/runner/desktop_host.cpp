#include "desktop_host.h"

#include <shellapi.h>

#include <flutter/standard_method_codec.h>

#include "resource.h"

namespace {

constexpr UINT kTrayIconId = 1;
constexpr UINT kTrayCallback = WM_APP + 1;
constexpr UINT kTrayMuteId = 1001;
constexpr UINT kTrayDeafenId = 1002;
constexpr UINT kTrayQuitId = 1003;

DesktopHost* g_desktop_host = nullptr;

std::wstring Utf8ToWide(const std::string& utf8) {
  if (utf8.empty()) {
    return std::wstring();
  }
  const int size =
      MultiByteToWideChar(CP_UTF8, 0, utf8.c_str(), -1, nullptr, 0);
  if (size <= 1) {
    return std::wstring();
  }
  std::wstring wide(static_cast<size_t>(size - 1), L'\0');
  MultiByteToWideChar(CP_UTF8, 0, utf8.c_str(), -1, wide.data(), size);
  return wide;
}

std::string StringArg(const flutter::EncodableMap& args, const char* key) {
  const auto it = args.find(flutter::EncodableValue(key));
  if (it == args.end()) {
    return std::string();
  }
  if (const auto* value = std::get_if<std::string>(&it->second)) {
    return *value;
  }
  return std::string();
}

bool BoolArg(const flutter::EncodableMap& args, const char* key) {
  const auto it = args.find(flutter::EncodableValue(key));
  if (it == args.end()) {
    return false;
  }
  if (const auto* value = std::get_if<bool>(&it->second)) {
    return *value;
  }
  return false;
}

int IntArg(const flutter::EncodableMap& args, const char* key) {
  const auto it = args.find(flutter::EncodableValue(key));
  if (it == args.end()) {
    return 0;
  }
  if (const auto* value = std::get_if<int32_t>(&it->second)) {
    return *value;
  }
  if (const auto* value = std::get_if<int64_t>(&it->second)) {
    return static_cast<int>(*value);
  }
  return 0;
}

LRESULT CALLBACK LowLevelKeyboardProc(int n_code, WPARAM wparam,
                                      LPARAM lparam) {
  if (n_code == HC_ACTION && g_desktop_host) {
    g_desktop_host->OnLowLevelKey(wparam, lparam);
  }
  return CallNextHookEx(nullptr, n_code, wparam, lparam);
}

}  // namespace

DesktopHost::DesktopHost(flutter::BinaryMessenger* messenger,
                         Win32Window* window)
    : window_(window) {
  g_desktop_host = this;
  channel_ = std::make_unique<flutter::MethodChannel<flutter::EncodableValue>>(
      messenger, "voice/windows_desktop",
      &flutter::StandardMethodCodec::GetInstance());
  channel_->SetMethodCallHandler(
      [this](const auto& call, auto result) {
        HandleMethodCall(call, std::move(result));
      });
  AddTrayIcon();
}

DesktopHost::~DesktopHost() {
  UnregisterPtt();
  RemoveTrayIcon();
  if (g_desktop_host == this) {
    g_desktop_host = nullptr;
  }
}

void DesktopHost::HandleMethodCall(
    const flutter::MethodCall<flutter::EncodableValue>& call,
    std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result) {
  const auto* args = std::get_if<flutter::EncodableMap>(call.arguments());
  if (call.method_name() == "setTrayState") {
    if (args) {
      UpdateTrayLabels(*args);
    }
    result->Success();
    return;
  }
  if (call.method_name() == "registerPttHotkey") {
    if (args) {
      RegisterPtt(IntArg(*args, "vkCode"), IntArg(*args, "modifiers"));
    }
    result->Success();
    return;
  }
  if (call.method_name() == "unregisterPttHotkey") {
    UnregisterPtt();
    result->Success();
    return;
  }
  if (call.method_name() == "showWindow") {
    ShowMainWindow();
    result->Success();
    return;
  }
  if (call.method_name() == "hideToTray") {
    HideToTray();
    result->Success();
    return;
  }
  if (call.method_name() == "quit") {
    QuitApp();
    result->Success();
    return;
  }
  result->NotImplemented();
}

void DesktopHost::UpdateTrayLabels(const flutter::EncodableMap& args) {
  muted_ = BoolArg(args, "muted");
  deafened_ = BoolArg(args, "deafened");
  const auto mute = StringArg(args, "muteLabel");
  const auto unmute = StringArg(args, "unmuteLabel");
  const auto deafen = StringArg(args, "deafenLabel");
  const auto undeafen = StringArg(args, "undeafenLabel");
  const auto quit = StringArg(args, "quitLabel");
  if (!mute.empty()) {
    mute_label_ = Utf8ToWide(mute);
  }
  if (!unmute.empty()) {
    unmute_label_ = Utf8ToWide(unmute);
  }
  if (!deafen.empty()) {
    deafen_label_ = Utf8ToWide(deafen);
  }
  if (!undeafen.empty()) {
    undeafen_label_ = Utf8ToWide(undeafen);
  }
  if (!quit.empty()) {
    quit_label_ = Utf8ToWide(quit);
  }
}

void DesktopHost::AddTrayIcon() {
  HWND hwnd = window_->GetHandle();
  if (!hwnd || tray_added_) {
    return;
  }
  nid_ = {};
  nid_.cbSize = sizeof(NOTIFYICONDATA);
  nid_.hWnd = hwnd;
  nid_.uID = kTrayIconId;
  nid_.uFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP;
  nid_.uCallbackMessage = kTrayCallback;
  nid_.hIcon = LoadIcon(GetModuleHandle(nullptr), MAKEINTRESOURCE(IDI_APP_ICON));
  wcsncpy_s(nid_.szTip, L"Voice", _TRUNCATE);
  tray_added_ = Shell_NotifyIcon(NIM_ADD, &nid_) == TRUE;
}

void DesktopHost::RemoveTrayIcon() {
  if (!tray_added_) {
    return;
  }
  Shell_NotifyIcon(NIM_DELETE, &nid_);
  tray_added_ = false;
}

void DesktopHost::ShowTrayMenu() {
  HWND hwnd = window_->GetHandle();
  if (!hwnd) {
    return;
  }
  POINT cursor;
  GetCursorPos(&cursor);
  HMENU menu = CreatePopupMenu();
  AppendMenu(menu, MF_STRING | (muted_ ? MF_CHECKED : MF_UNCHECKED), kTrayMuteId,
             muted_ ? unmute_label_.c_str() : mute_label_.c_str());
  AppendMenu(menu, MF_STRING | (deafened_ ? MF_CHECKED : MF_UNCHECKED),
             kTrayDeafenId,
             deafened_ ? undeafen_label_.c_str() : deafen_label_.c_str());
  AppendMenu(menu, MF_SEPARATOR, 0, nullptr);
  AppendMenu(menu, MF_STRING, kTrayQuitId, quit_label_.c_str());
  SetForegroundWindow(hwnd);
  TrackPopupMenu(menu, TPM_RIGHTBUTTON, cursor.x, cursor.y, 0, hwnd, nullptr);
  DestroyMenu(menu);
}

void DesktopHost::HideToTray() {
  if (HWND hwnd = window_->GetHandle()) {
    ShowWindow(hwnd, SW_HIDE);
  }
}

void DesktopHost::ShowMainWindow() {
  if (HWND hwnd = window_->GetHandle()) {
    ShowWindow(hwnd, SW_SHOWNORMAL);
    SetForegroundWindow(hwnd);
  }
}

void DesktopHost::QuitApp() {
  UnregisterPtt();
  RemoveTrayIcon();
  window_->SetQuitOnClose(true);
  if (HWND hwnd = window_->GetHandle()) {
    DestroyWindow(hwnd);
  } else {
    PostQuitMessage(0);
  }
}

void DesktopHost::RegisterPtt(int vk_code, int modifiers) {
  ptt_vk_ = vk_code;
  ptt_modifiers_ = modifiers;
  if (!hook_) {
    hook_ = SetWindowsHookEx(WH_KEYBOARD_LL, LowLevelKeyboardProc, nullptr, 0);
  }
}

void DesktopHost::UnregisterPtt() {
  ptt_vk_ = 0;
  ptt_modifiers_ = 0;
  ptt_held_ = false;
  if (hook_) {
    UnhookWindowsHookEx(hook_);
    hook_ = nullptr;
  }
}

void DesktopHost::OnLowLevelKey(WPARAM wparam, LPARAM lparam) {
  if (ptt_vk_ == 0) {
    return;
  }
  const auto* info = reinterpret_cast<KBDLLHOOKSTRUCT*>(lparam);
  if (info == nullptr || static_cast<int>(info->vkCode) != ptt_vk_) {
    return;
  }
  const bool ctrl = (GetAsyncKeyState(VK_CONTROL) & 0x8000) != 0;
  const bool alt = (GetAsyncKeyState(VK_MENU) & 0x8000) != 0;
  const bool shift = (GetAsyncKeyState(VK_SHIFT) & 0x8000) != 0;
  if ((ptt_modifiers_ & 1) && !ctrl) {
    return;
  }
  if ((ptt_modifiers_ & 2) && !alt) {
    return;
  }
  if ((ptt_modifiers_ & 4) && !shift) {
    return;
  }
  const bool down = wparam == WM_KEYDOWN || wparam == WM_SYSKEYDOWN;
  const bool up = wparam == WM_KEYUP || wparam == WM_SYSKEYUP;
  if (down && !ptt_held_) {
    ptt_held_ = true;
    Emit("ptt", flutter::EncodableValue(flutter::EncodableMap{
                    {flutter::EncodableValue("held"),
                     flutter::EncodableValue(true)}}));
  } else if (up && ptt_held_) {
    ptt_held_ = false;
    Emit("ptt", flutter::EncodableValue(flutter::EncodableMap{
                    {flutter::EncodableValue("held"),
                     flutter::EncodableValue(false)}}));
  }
}

void DesktopHost::Emit(const std::string& method, flutter::EncodableValue args) {
  if (!channel_) {
    return;
  }
  channel_->InvokeMethod(method, std::make_unique<flutter::EncodableValue>(
                                     std::move(args)));
}

LRESULT DesktopHost::HandleMessage(HWND hwnd, UINT message, WPARAM wparam,
                                   LPARAM lparam) {
  if (message == kTrayCallback) {
    if (lparam == WM_RBUTTONUP) {
      ShowTrayMenu();
    } else if (lparam == WM_LBUTTONUP || lparam == WM_LBUTTONDBLCLK) {
      ShowMainWindow();
      Emit("trayShow", flutter::EncodableValue());
    }
    return 0;
  }
  if (message == WM_COMMAND) {
    switch (LOWORD(wparam)) {
      case kTrayMuteId:
        Emit("trayMute", flutter::EncodableValue());
        return 0;
      case kTrayDeafenId:
        Emit("trayDeafen", flutter::EncodableValue());
        return 0;
      case kTrayQuitId:
        Emit("trayQuit", flutter::EncodableValue());
        return 0;
      default:
        break;
    }
  }
  return 0;
}
