#ifndef RUNNER_DESKTOP_HOST_H_
#define RUNNER_DESKTOP_HOST_H_

#include <flutter/binary_messenger.h>
#include <flutter/encodable_value.h>
#include <flutter/method_channel.h>

#include <memory>
#include <string>

#include "win32_window.h"

// Windows tray + global PTT (docs/features/platforms.md П.17).
class DesktopHost {
 public:
  DesktopHost(flutter::BinaryMessenger* messenger, Win32Window* window);
  ~DesktopHost();

  LRESULT HandleMessage(HWND hwnd, UINT message, WPARAM wparam,
                        LPARAM lparam);
  void OnLowLevelKey(WPARAM wparam, LPARAM lparam);

 private:
  void HandleMethodCall(
      const flutter::MethodCall<flutter::EncodableValue>& call,
      std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result);
  void AddTrayIcon();
  void RemoveTrayIcon();
  void ShowTrayMenu();
  void HideToTray();
  void ShowMainWindow();
  void QuitApp();
  void RegisterPtt(int vk_code, int modifiers);
  void UnregisterPtt();
  void Emit(const std::string& method, flutter::EncodableValue args);
  void UpdateTrayLabels(const flutter::EncodableMap& args);

  Win32Window* window_;
  std::unique_ptr<flutter::MethodChannel<flutter::EncodableValue>> channel_;
  NOTIFYICONDATA nid_{};
  bool tray_added_ = false;
  bool muted_ = false;
  bool deafened_ = false;
  std::wstring mute_label_ = L"Mute";
  std::wstring unmute_label_ = L"Unmute";
  std::wstring deafen_label_ = L"Deafen";
  std::wstring undeafen_label_ = L"Undeafen";
  std::wstring quit_label_ = L"Quit";
  int ptt_vk_ = 0;
  int ptt_modifiers_ = 0;
  bool ptt_held_ = false;
  HHOOK hook_ = nullptr;
};

#endif  // RUNNER_DESKTOP_HOST_H_
