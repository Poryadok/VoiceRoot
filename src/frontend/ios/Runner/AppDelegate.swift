import CallKit
import Flutter
import PushKit
import UIKit

@main
@objc class AppDelegate: FlutterAppDelegate, FlutterImplicitEngineDelegate, PKPushRegistryDelegate, CXProviderDelegate {
  private var voipRegistry: PKPushRegistry?
  private var callProvider: CXProvider?
  private var voipChannel: FlutterMethodChannel?
  private var pendingCalls: [UUID: [String: Any]] = [:]

  override func application(
    _ application: UIApplication,
    didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
  ) -> Bool {
    return super.application(application, didFinishLaunchingWithOptions: launchOptions)
  }

  func didInitializeImplicitFlutterEngine(_ engineBridge: FlutterImplicitEngineBridge) {
    GeneratedPluginRegistrant.register(with: engineBridge.pluginRegistry)
    let registrar = engineBridge.pluginRegistry.registrar(forPlugin: "VoiceVoIP")!
    voipChannel = FlutterMethodChannel(name: "voice/voip", binaryMessenger: registrar.messenger())
    voipChannel?.setMethodCallHandler { [weak self] call, result in
      switch call.method {
      case "stop":
        self?.pendingCalls.removeAll()
        result(nil)
      default:
        result(FlutterMethodNotImplemented)
      }
    }

    voipRegistry = PKPushRegistry(queue: DispatchQueue.main)
    voipRegistry?.delegate = self
    voipRegistry?.desiredPushTypes = [.voIP]

    let config = CXProviderConfiguration(localizedName: "Voice")
    config.supportsVideo = true
    config.includesCallsInRecents = false
    callProvider = CXProvider(configuration: config)
    callProvider?.setDelegate(self, queue: nil)
  }

  func pushRegistry(
    _ registry: PKPushRegistry,
    didUpdate pushCredentials: PKPushCredentials,
    for type: PKPushType
  ) {
    let token = pushCredentials.token.map { String(format: "%02x", $0) }.joined()
    voipChannel?.invokeMethod("onVoIPToken", arguments: token)
  }

  func pushRegistry(
    _ registry: PKPushRegistry,
    didReceiveIncomingPushWith payload: PKPushPayload,
    for type: PKPushType,
    completion: @escaping () -> Void
  ) {
    let payloadMap = normalizeVoIPPayload(payload.dictionaryPayload)
    let callUUID = UUID()
    pendingCalls[callUUID] = payloadMap

    let update = CXCallUpdate()
    let callerId = payloadMap["initiator_profile_id"] as? String ?? "caller"
    update.remoteHandle = CXHandle(type: .generic, value: callerId)
    update.localizedCallerName = callerId
    update.hasVideo = (payloadMap["media_kind"] as? String)?.lowercased() == "video"

    callProvider?.reportNewIncomingCall(with: callUUID, update: update) { [weak self] error in
      if error == nil {
        self?.voipChannel?.invokeMethod("onIncomingCall", arguments: payloadMap)
      }
      completion()
    }
  }

  func provider(_ provider: CXProvider, perform action: CXAnswerCallAction) {
    if let data = pendingCalls[action.callUUID] {
      voipChannel?.invokeMethod("onCallAccepted", arguments: data)
    }
    pendingCalls.removeValue(forKey: action.callUUID)
    action.fulfill()
  }

  func provider(_ provider: CXProvider, perform action: CXEndCallAction) {
    if let data = pendingCalls[action.callUUID] {
      voipChannel?.invokeMethod("onCallDeclined", arguments: data)
    }
    pendingCalls.removeValue(forKey: action.callUUID)
    action.fulfill()
  }

  private func normalizeVoIPPayload(_ raw: [AnyHashable: Any]) -> [String: Any] {
    var out: [String: Any] = [:]
    for (key, value) in raw {
      guard let key = key as? String else { continue }
      if key == "aps" { continue }
      out[key] = value
    }
    return out
  }
}
