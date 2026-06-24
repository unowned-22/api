package uaparser

import "strings"

type DeviceInfo struct {
	DeviceType string // "mobile" | "desktop" | "unknown"
	OS         string // "iOS 18", "Android 15", "macOS 14", "Windows 10", ""
	Browser    string // "Safari", "Chrome", "Firefox", "Edge", ""
}

// Parse performs minimal heuristics to extract device type, OS and browser
func Parse(ua string) DeviceInfo {
	uaLower := strings.ToLower(ua)
	di := DeviceInfo{DeviceType: "unknown"}

	// device type
	if strings.Contains(uaLower, "mobile") || strings.Contains(uaLower, "iphone") || strings.Contains(uaLower, "android") {
		di.DeviceType = "mobile"
	} else if strings.Contains(uaLower, "windows") || strings.Contains(uaLower, "macintosh") || strings.Contains(uaLower, "mac os") || strings.Contains(uaLower, "linux") {
		di.DeviceType = "desktop"
	}

	// browser
	switch {
	case strings.Contains(uaLower, "chrome") && !strings.Contains(uaLower, "edge"):
		di.Browser = "Chrome"
	case strings.Contains(uaLower, "firefox"):
		di.Browser = "Firefox"
	case strings.Contains(uaLower, "safari") && !strings.Contains(uaLower, "chrome"):
		di.Browser = "Safari"
	case strings.Contains(uaLower, "edge"):
		di.Browser = "Edge"
	default:
		di.Browser = ""
	}

	// OS (simple heuristics)
	if strings.Contains(uaLower, "iphone os") || strings.Contains(uaLower, "ipad; cpu os") || strings.Contains(uaLower, "iphone") {
		di.OS = "iOS"
	} else if strings.Contains(uaLower, "android") {
		di.OS = "Android"
	} else if strings.Contains(uaLower, "mac os x") || strings.Contains(uaLower, "macintosh") {
		di.OS = "macOS"
	} else if strings.Contains(uaLower, "windows nt") {
		di.OS = "Windows"
	} else if strings.Contains(uaLower, "linux") {
		di.OS = "Linux"
	}

	return di
}
