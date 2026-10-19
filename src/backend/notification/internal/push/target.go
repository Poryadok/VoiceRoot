package push

import "errors"

// MobileTarget validates the canonical stored platform/provider pair. A release
// guard must never interpret unknown metadata as an FCM target.
func MobileTarget(platform, provider string) (bool, error) {
	switch platform {
	case "android":
		if provider == "fcm" {
			return true, nil
		}
	case "ios":
		if provider == "fcm" || provider == "apns" || provider == "voip_apns" {
			return true, nil
		}
	case "web", "desktop":
		if provider == "fcm" {
			return false, nil
		}
	}
	return false, errors.New("invalid push target metadata")
}
